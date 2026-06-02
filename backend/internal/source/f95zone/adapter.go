package f95zone

import (
	stdhtml "html"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"local/ludex/internal/domain"
)

var (
	bracketRE      = regexp.MustCompile(`\[[^\]]+\]`)
	threadIDRE     = regexp.MustCompile(`(?i)(?:threads?/[^./]+\.|threads?/[^/]+/)?(\d{3,})(?:/|$)`)
	versionLikeRE  = regexp.MustCompile(`(?i)\b(?:v(?:ersion)?\.?\s*)?\d+(?:\.\d+){0,4}[a-z0-9._ -]*\b|alpha|beta|demo|chapter\s*\d+|episode\s*\d+`)
	labelRE        = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 /_.-]{1,42}):\s*(.+)$`)
	downloadLineRE = regexp.MustCompile(`(?i)^([A-Za-z][A-Za-z/ +().0-9-]{1,48}):\s*(.*)$`)
	platformNoteRE = regexp.MustCompile(`^(.+?)\s*\(([^)]*)\)\s*$`)
	spaceRE        = regexp.MustCompile(`\s+`)
)

var unavailableText = "You don't have permission to view the spoiler content."

var sectionLabels = []string{
	"Overview", "Story", "Description", "Changelog", "Change Log", "Installation",
	"Developer Notes", "Features", "Controls", "System Requirements", "Fan Signatures",
	"Download", "Downloads",
}

type richLine struct {
	Text  string
	Links []domain.NamedURL
}

func ParseHTML(rawURL string, r io.Reader) (domain.Transcript, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return domain.Transcript{}, err
	}

	title, prefixes := extractThreadTitle(doc)
	title = firstNonEmpty(
		title,
		cleanText(doc.Find("meta[property='og:title']").AttrOr("content", "")),
		cleanText(doc.Find("title").First().Text()),
	)
	title = strings.TrimSuffix(title, " | F95zone")
	title = stripKnownPrefixes(title, prefixes)

	body := firstPostBody(doc)
	lines := readableLines(body)
	keyValues := extractKeyValues(lines)
	sections := extractSections(lines)
	images := extractImages(doc, body)
	tags := extractTags(doc)
	tags = uniqueStrings(append(prefixes, tags...))
	fields := extractFields(title, prefixes, keyValues, sections, tags, images, lines, body)

	transcript := domain.Transcript{
		Source:     "f95zone",
		SourceURL:  rawURL,
		ExternalID: InferExternalID(rawURL),
		Title:      title,
		Fields:     fields,
		KeyValues:  keyValues,
		Sections:   sections,
		Images:     images,
		Tags:       tags,
	}
	transcript.Inferred = inferMeta(transcript)
	if transcript.Title == "" {
		transcript.Warnings = append(transcript.Warnings, "Could not find a thread title in the supplied HTML.")
	}
	if len(transcript.Sections) == 0 && len(transcript.KeyValues) == 0 {
		transcript.Warnings = append(transcript.Warnings, "Could not find structured labels in the first post; the page may require login or use an unsupported layout.")
	}

	return transcript, nil
}

func extractThreadTitle(doc *goquery.Document) (string, []string) {
	h1 := doc.Find("h1.p-title-value").First()
	prefixes := []string{}
	h1.Find("a.labelLink").Each(func(_ int, s *goquery.Selection) {
		if value := cleanText(s.Text()); value != "" {
			prefixes = append(prefixes, value)
		}
	})
	if h1.Length() == 0 {
		return "", uniqueStrings(prefixes)
	}
	clone := h1.Clone()
	clone.Find("a.labelLink, .label-append").Remove()
	return cleanText(clone.Text()), uniqueStrings(prefixes)
}

func firstPostBody(doc *goquery.Document) *goquery.Selection {
	selectors := []string{
		"article.message--post .bbWrapper",
		".message--post .message-body",
		".message-userContent .bbWrapper",
		".message-body",
		".bbWrapper",
	}
	for _, selector := range selectors {
		if sel := doc.Find(selector).First(); sel.Length() > 0 {
			return sel
		}
	}
	return doc.Selection
}

func readableLines(sel *goquery.Selection) []string {
	text := strings.ReplaceAll(sel.Text(), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = cleanText(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func extractKeyValues(lines []string) map[string][]string {
	out := map[string][]string{}
	for _, line := range lines {
		match := labelRE.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		key := normalizeLabel(match[1])
		value := strings.TrimSpace(match[2])
		if value == "" || len(value) > 800 {
			continue
		}
		out[key] = append(out[key], value)
	}
	return out
}

func extractSections(lines []string) []domain.TranscriptSection {
	sections := []domain.TranscriptSection{}
	current := ""
	buffer := []string{}

	flush := func() {
		if current == "" || len(buffer) == 0 {
			buffer = nil
			return
		}
		sections = append(sections, domain.TranscriptSection{
			Heading: current,
			Body:    strings.Join(buffer, "\n"),
		})
		buffer = nil
	}

	for _, line := range lines {
		heading, inline := splitSectionHeading(line)
		if heading != "" {
			flush()
			current = heading
			if inline != "" {
				buffer = append(buffer, inline)
			}
			continue
		}
		if current != "" {
			if labelRE.MatchString(line) && len(buffer) > 0 {
				flush()
				current = ""
				continue
			}
			buffer = append(buffer, line)
		}
	}
	flush()
	return sections
}

func splitSectionHeading(line string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(line))
	for _, label := range sectionLabels {
		want := strings.ToLower(label)
		if lower == want || lower == want+":" {
			return label, ""
		}
		if strings.HasPrefix(lower, want+":") {
			return label, strings.TrimSpace(line[len(label)+1:])
		}
	}
	return "", ""
}

func extractSectionHTML(body *goquery.Selection, headings ...string) string {
	targets := map[string]bool{}
	for _, heading := range headings {
		targets[strings.ToLower(normalizeLabel(heading))] = true
	}

	started := false
	pieces := []string{}
	body.Contents().EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		if len(sel.Nodes) == 0 {
			return true
		}
		node := sel.Nodes[0]
		text := cleanText(sel.Text())
		heading, inline := splitSectionHeading(text)
		normalizedHeading := strings.ToLower(normalizeLabel(heading))
		if !started {
			if heading != "" && targets[normalizedHeading] {
				started = true
				if inline != "" {
					pieces = append(pieces, stdhtml.EscapeString(cleanUnavailable(inline)))
				}
			}
			return true
		}
		if len(pieces) == 0 && isLeadingSectionJunk(node) {
			return true
		}
		if heading != "" {
			return false
		}
		pieces = append(pieces, sanitizeHTMLNode(node))
		return true
	})

	out := cleanSanitizedHTML(strings.Join(pieces, ""))
	if strings.Contains(out, stdhtml.EscapeString(unavailableText)) {
		return ""
	}
	return out
}

func isLeadingSectionJunk(node *html.Node) bool {
	if node == nil {
		return true
	}
	if node.Type == html.TextNode {
		return strings.Trim(strings.TrimSpace(node.Data), ":") == ""
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "br") {
		return true
	}
	return false
}

func sanitizeHTMLNode(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		if strings.TrimSpace(node.Data) == "" {
			return ""
		}
		return stdhtml.EscapeString(node.Data)
	}
	if node.Type != html.ElementNode {
		return sanitizeHTMLChildren(node)
	}

	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "iframe", "object", "embed", "button", "img", "noscript":
		return ""
	case "br":
		return "<br>"
	case "b", "strong":
		return wrapSanitizedHTML("strong", sanitizeHTMLChildren(node))
	case "i", "em":
		return wrapSanitizedHTML("em", sanitizeHTMLChildren(node))
	case "u":
		return wrapSanitizedHTML("u", sanitizeHTMLChildren(node))
	case "s", "strike", "del":
		return wrapSanitizedHTML("s", sanitizeHTMLChildren(node))
	case "ul", "ol", "li", "blockquote", "code", "pre":
		return wrapSanitizedHTML(tag, sanitizeHTMLChildren(node))
	case "p":
		return wrapSanitizedHTML("p", sanitizeHTMLChildren(node))
	case "a":
		body := sanitizeHTMLChildren(node)
		href := strings.TrimSpace(nodeAttr(node, "href"))
		if !isSafeHTMLHref(href) {
			return body
		}
		return `<a href="` + stdhtml.EscapeString(href) + `" target="_blank" rel="noreferrer">` + body + `</a>`
	default:
		return sanitizeHTMLChildren(node)
	}
}

func sanitizeHTMLChildren(node *html.Node) string {
	if node == nil {
		return ""
	}
	parts := []string{}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if html := sanitizeHTMLNode(child); html != "" {
			parts = append(parts, html)
		}
	}
	return strings.Join(parts, "")
}

func wrapSanitizedHTML(tag string, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return "<" + tag + ">" + body + "</" + tag + ">"
}

func isSafeHTMLHref(value string) bool {
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.IsAbs() {
		return parsed.Scheme == "http" || parsed.Scheme == "https"
	}
	return strings.HasPrefix(value, "/")
}

func cleanSanitizedHTML(value string) string {
	value = strings.ReplaceAll(value, stdhtml.EscapeString(unavailableText), "")
	value = strings.ReplaceAll(value, "Log in or register now.", "")
	return strings.TrimSpace(value)
}

func extractImages(doc *goquery.Document, body *goquery.Selection) []string {
	seen := map[string]bool{}
	images := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(value, "data:") || seen[value] {
			return
		}
		seen[value] = true
		images = append(images, value)
	}

	body.Find("img.bbImage, img").Each(func(_ int, s *goquery.Selection) {
		value := firstNonEmpty(
			s.Closest("a").AttrOr("href", ""),
			s.AttrOr("data-src", ""),
			s.AttrOr("data-url", ""),
			s.AttrOr("src", ""),
		)
		if isContentImage(value, s) {
			add(value)
		}
	})
	if len(images) == 0 {
		doc.Find("meta[property='og:image']").Each(func(_ int, s *goquery.Selection) {
			value := s.AttrOr("content", "")
			if isContentImage(value, s) {
				add(value)
			}
		})
	}
	if len(images) > 50 {
		return images[:50]
	}
	return images
}

func isContentImage(value string, s *goquery.Selection) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || strings.HasPrefix(value, "data:") {
		return false
	}
	class := strings.ToLower(s.AttrOr("class", ""))
	if strings.Contains(class, "smilie") || strings.Contains(class, "avatar") {
		return false
	}
	if strings.Contains(value, "favicon") || strings.Contains(value, "/styles/") || strings.Contains(value, "/assets/") {
		return false
	}
	return strings.Contains(class, "bbimage") || strings.Contains(value, "attachments.f95zone.to") || strings.Contains(value, "/attachments/")
}

func extractTags(doc *goquery.Document) []string {
	seen := map[string]bool{}
	tags := []string{}
	doc.Find(".js-tagList a, a.tagItem, .tagList a").Each(func(_ int, s *goquery.Selection) {
		tag := cleanText(s.Text())
		if tag != "" && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	})
	return tags
}

func extractFields(title string, prefixes []string, keyValues map[string][]string, sections []domain.TranscriptSection, tags []string, images []string, lines []string, body *goquery.Selection) domain.TranscriptFields {
	description := cleanUnavailable(firstSection(sections, "Overview", "Story", "Description"))
	rawChangelog := firstSection(sections, "Changelog", "Change Log")
	changelog := ""
	changelogHTML := ""
	if !strings.Contains(rawChangelog, unavailableText) {
		changelog = cleanUnavailable(rawChangelog)
		changelogHTML = extractSectionHTML(body, "Changelog", "Change Log")
	}

	fields := domain.TranscriptFields{
		GameName:         inferGameTitle(title),
		Prefixes:         prefixes,
		Engine:           inferEngine(prefixes),
		Description:      description,
		ThreadUpdated:    firstKeyValue(keyValues, "Thread Updated"),
		ReleaseDate:      firstKeyValue(keyValues, "Release Date"),
		Developer:        cleanDeveloper(firstKeyValue(keyValues, "Developer", "Publisher")),
		DeveloperLinks:   extractLabelLinks(body, "Developer"),
		Version:          firstKeyValue(keyValues, "Version"),
		OperatingSystems: splitList(firstKeyValue(keyValues, "OS", "Os", "Operating System", "Operating Systems")),
		Languages:        uniqueStrings(append(splitList(firstKeyValue(keyValues, "Language")), splitList(firstKeyValue(keyValues, "Languages"))...)),
		Genres:           splitList(firstKeyValue(keyValues, "Genre", "Genres")),
		Changelog:        changelog,
		ChangelogHTML:    changelogHTML,
		DownloadGroups:   extractDownloadGroups(body, lines),
	}
	if fields.Version == "" {
		fields.Version = inferVersionFromTitle(title)
	}
	if fields.Developer == "" {
		fields.Developer = inferDeveloperFromTitle(title)
	}
	if value, ok := parseYesNo(firstKeyValue(keyValues, "Censored")); ok {
		fields.Censored = &value
	}
	if len(fields.Genres) == 0 {
		fields.Genres = inferGenresFromTags(tags, prefixes)
	}
	if len(images) > 0 {
		fields.CoverImage = images[0]
	}
	if len(images) > 1 {
		fields.Screenshots = append([]string{}, images[1:]...)
	}
	return fields
}

func extractLabelLinks(body *goquery.Selection, label string) []domain.NamedURL {
	links := []domain.NamedURL{}
	seen := map[string]bool{}
	normalizedLabel := normalizeLabel(label)

	body.Find("b").EachWithBreak(func(_ int, b *goquery.Selection) bool {
		if normalizeLabel(cleanText(b.Text())) != normalizedLabel || len(b.Nodes) == 0 {
			return true
		}
		foundLabel := false
		b.Parent().Contents().EachWithBreak(func(_ int, nodeSel *goquery.Selection) bool {
			if len(nodeSel.Nodes) == 0 {
				return true
			}
			node := nodeSel.Nodes[0]
			if node == b.Nodes[0] {
				foundLabel = true
				return true
			}
			if !foundLabel {
				return true
			}
			if node.Type == html.ElementNode && strings.EqualFold(node.Data, "br") {
				return false
			}
			links = collectLinks(nodeSel, links, seen)
			return true
		})
		return false
	})
	return links
}

func collectLinks(sel *goquery.Selection, links []domain.NamedURL, seen map[string]bool) []domain.NamedURL {
	add := func(link *goquery.Selection) {
		href := strings.TrimSpace(link.AttrOr("href", ""))
		name := cleanText(link.Text())
		if href == "" || name == "" || strings.Contains(name, "You must be registered") || seen[href] {
			return
		}
		seen[href] = true
		links = append(links, domain.NamedURL{Name: name, URL: href})
	}
	if sel.Is("a") {
		add(sel)
	}
	sel.Find("a").Each(func(_ int, link *goquery.Selection) {
		add(link)
	})
	return links
}

func extractDownloadGroups(body *goquery.Selection, lines []string) []domain.DownloadGroup {
	if groups := extractDownloadGroupsFromRichLines(downloadRichLines(body)); len(groups) > 0 {
		return groups
	}
	return extractDownloadGroupsFromLines(lines)
}

func downloadRichLines(body *goquery.Selection) []richLine {
	lines := []richLine{}
	current := richLine{}

	appendText := func(value string) {
		value = cleanText(value)
		if value == "" {
			return
		}
		current.Text = cleanText(strings.TrimSpace(current.Text + " " + value))
	}
	flush := func() {
		current.Text = cleanText(current.Text)
		if current.Text != "" || len(current.Links) > 0 {
			lines = append(lines, current)
		}
		current = richLine{}
	}

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			appendText(node.Data)
			return
		}
		if node.Type != html.ElementNode {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
			return
		}
		switch strings.ToLower(node.Data) {
		case "br":
			flush()
			return
		case "a":
			name := cleanText(nodeText(node))
			href := strings.TrimSpace(nodeAttr(node, "href"))
			appendText(name)
			if href != "" && name != "" {
				current.Links = append(current.Links, domain.NamedURL{Name: name, URL: href})
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		switch strings.ToLower(node.Data) {
		case "div", "p", "li", "blockquote":
			flush()
		}
	}

	for _, node := range body.Nodes {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	flush()
	return lines
}

func extractDownloadGroupsFromRichLines(lines []richLine) []domain.DownloadGroup {
	groups := []domain.DownloadGroup{}
	inDownloads := false
	for _, line := range lines {
		text := cleanText(strings.Trim(line.Text, "[]"))
		if text == "" {
			continue
		}
		upper := strings.ToUpper(text)
		if upper == "DOWNLOAD" || upper == "DOWNLOADS" {
			inDownloads = true
			continue
		}
		if !inDownloads {
			continue
		}
		if strings.HasPrefix(upper, "PATCHES") || strings.HasPrefix(upper, "EXTRAS") || strings.HasPrefix(upper, "LANGUAGES") || strings.HasPrefix(upper, "* ") {
			break
		}
		match := downloadLineRE.FindStringSubmatch(text)
		if len(match) != 3 {
			continue
		}
		platform, note := splitPlatformNote(cleanText(match[1]))
		if !looksLikePlatform(platform) {
			continue
		}
		group := domain.DownloadGroup{
			Platform: platform,
			Note:     note,
			Links:    downloadLinksForLine(line, match[2]),
		}
		groups = append(groups, group)
	}
	return groups
}

func splitPlatformNote(value string) (string, string) {
	match := platformNoteRE.FindStringSubmatch(value)
	if len(match) != 3 {
		return value, ""
	}
	return cleanText(match[1]), cleanText(match[2])
}

func extractDownloadGroupsFromLines(lines []string) []domain.DownloadGroup {
	richLines := make([]richLine, 0, len(lines))
	for _, line := range lines {
		richLines = append(richLines, richLine{Text: line})
	}
	return extractDownloadGroupsFromRichLines(richLines)
}

func downloadLinksForLine(line richLine, fallbackText string) []domain.NamedURL {
	seen := map[string]bool{}
	links := []domain.NamedURL{}
	for _, link := range line.Links {
		name := cleanText(strings.Trim(link.Name, "* "))
		url := strings.TrimSpace(link.URL)
		if name == "" || url == "" || strings.Contains(strings.ToLower(name), "registered") || seen[url] {
			continue
		}
		seen[url] = true
		links = append(links, domain.NamedURL{Name: name, URL: url})
	}
	if len(links) > 0 {
		return links
	}
	return splitDownloadNames(fallbackText)
}

func splitDownloadNames(value string) []domain.NamedURL {
	value = cleanUnavailable(value)
	if value == "" {
		return nil
	}
	parts := regexp.MustCompile(`\s+-\s+|,\s*`).Split(value, -1)
	links := []domain.NamedURL{}
	for _, part := range parts {
		name := cleanText(strings.Trim(part, "* "))
		if name != "" && !strings.Contains(name, "registered") {
			links = append(links, domain.NamedURL{Name: name})
		}
	}
	return links
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	parts := []string{}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if text := nodeText(child); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func nodeAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func looksLikePlatform(value string) bool {
	lower := strings.ToLower(value)
	for _, token := range []string{"win", "linux", "mac", "android", "ios", "pc"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func splitList(value string) []string {
	value = cleanUnavailable(value)
	if value == "" {
		return nil
	}
	parts := regexp.MustCompile(`\s*,\s*|\s+-\s+`).Split(value, -1)
	out := []string{}
	for _, part := range parts {
		part = cleanText(strings.Trim(part, "* "))
		if part != "" {
			out = append(out, part)
		}
	}
	return uniqueStrings(out)
}

func parseYesNo(value string) (bool, bool) {
	switch strings.ToLower(cleanText(value)) {
	case "yes", "true", "censored":
		return true, true
	case "no", "false", "uncensored":
		return false, true
	default:
		return false, false
	}
}

func cleanUnavailable(value string) string {
	value = strings.ReplaceAll(value, unavailableText, "")
	value = strings.ReplaceAll(value, "Log in or register now.", "")
	return cleanText(value)
}

func cleanDeveloper(value string) string {
	value = cleanUnavailable(value)
	for _, suffix := range []string{" Patreon", " F95zone", " Discord"} {
		value = strings.ReplaceAll(value, suffix, "")
	}
	return strings.Trim(value, " -")
}

func inferEngine(prefixes []string) string {
	for _, prefix := range prefixes {
		if strings.EqualFold(prefix, "Ren'Py") || strings.EqualFold(prefix, "Unity") || strings.EqualFold(prefix, "RPGM") || strings.EqualFold(prefix, "Unreal Engine") {
			return prefix
		}
	}
	return ""
}

func inferGenresFromTags(tags []string, prefixes []string) []string {
	out := []string{}
	for _, tag := range tags {
		if slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.EqualFold(prefix, tag) }) {
			continue
		}
		out = append(out, tag)
	}
	return uniqueStrings(out)
}

func inferMeta(t domain.Transcript) domain.TranscriptInferred {
	inferred := domain.TranscriptInferred{
		GameTitle:   firstNonEmpty(t.Fields.GameName, inferGameTitle(t.Title)),
		Version:     firstNonEmpty(t.Fields.Version, firstKeyValue(t.KeyValues, "Version")),
		Developer:   firstNonEmpty(t.Fields.Developer, firstKeyValue(t.KeyValues, "Developer", "Publisher")),
		Description: firstNonEmpty(t.Fields.Description, firstSection(t.Sections, "Overview", "Story", "Description")),
	}
	if inferred.Version == "" {
		inferred.Version = inferVersionFromTitle(t.Title)
	}
	if inferred.Developer == "" {
		inferred.Developer = inferDeveloperFromTitle(t.Title)
	}
	if t.Fields.CoverImage != "" {
		inferred.CoverImage = t.Fields.CoverImage
	} else if len(t.Images) > 0 {
		inferred.CoverImage = t.Images[0]
	}
	return inferred
}

func inferGameTitle(title string) string {
	title = strings.TrimSuffix(title, " | F95zone")
	title = bracketRE.ReplaceAllString(title, "")
	title = strings.Trim(title, " -\t")
	return cleanText(title)
}

func stripKnownPrefixes(title string, prefixes []string) string {
	title = strings.TrimSuffix(title, " | F95zone")
	for _, prefix := range prefixes {
		title = strings.TrimSpace(strings.TrimPrefix(title, prefix+" - "))
		title = strings.TrimSpace(strings.TrimPrefix(title, prefix))
	}
	return strings.Trim(title, " -\t")
}

func inferVersionFromTitle(title string) string {
	for _, match := range bracketRE.FindAllString(title, -1) {
		value := strings.Trim(match, "[] ")
		if versionLikeRE.MatchString(value) {
			return value
		}
	}
	return ""
}

func inferDeveloperFromTitle(title string) string {
	matches := bracketRE.FindAllString(title, -1)
	if len(matches) == 0 {
		return ""
	}
	for i := len(matches) - 1; i >= 0; i-- {
		value := strings.Trim(matches[i], "[] ")
		if value != "" && !versionLikeRE.MatchString(value) {
			return value
		}
	}
	return ""
}

func InferExternalID(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		rawURL = parsed.Path
	}
	match := threadIDRE.FindStringSubmatch(rawURL)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func firstKeyValue(values map[string][]string, keys ...string) string {
	for _, key := range keys {
		for actual, list := range values {
			if strings.EqualFold(actual, key) && len(list) > 0 {
				return list[0]
			}
		}
	}
	return ""
}

func firstSection(sections []domain.TranscriptSection, headings ...string) string {
	for _, heading := range headings {
		for _, section := range sections {
			if strings.EqualFold(section.Heading, heading) {
				return section.Body
			}
		}
	}
	return ""
}

func normalizeLabel(label string) string {
	label = cleanText(label)
	parts := strings.Fields(label)
	for i := range parts {
		if len(parts[i]) > 1 {
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}
	return strings.Join(parts, " ")
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.TrimSpace(spaceRE.ReplaceAllString(value, " "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}
