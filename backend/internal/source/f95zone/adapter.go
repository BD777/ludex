package f95zone

import (
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"local/ludex/internal/domain"
)

var (
	bracketRE     = regexp.MustCompile(`\[[^\]]+\]`)
	threadIDRE    = regexp.MustCompile(`(?i)(?:threads?/[^./]+\.|threads?/[^/]+/)?(\d{3,})(?:/|$)`)
	versionLikeRE = regexp.MustCompile(`(?i)\b(?:v(?:ersion)?\.?\s*)?\d+(?:\.\d+){0,4}[a-z0-9._ -]*\b|alpha|beta|demo|chapter\s*\d+|episode\s*\d+`)
	labelRE       = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 /_.-]{1,42}):\s*(.+)$`)
	spaceRE       = regexp.MustCompile(`\s+`)
)

var sectionLabels = []string{
	"Overview", "Story", "Description", "Changelog", "Change Log", "Installation",
	"Developer Notes", "Features", "Controls", "System Requirements",
}

func ParseHTML(rawURL string, r io.Reader) (domain.Transcript, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return domain.Transcript{}, err
	}

	title := firstNonEmpty(
		cleanText(doc.Find("h1.p-title-value").First().Text()),
		cleanText(doc.Find("meta[property='og:title']").AttrOr("content", "")),
		cleanText(doc.Find("title").First().Text()),
	)
	title = strings.TrimSuffix(title, " | F95zone")

	body := firstPostBody(doc)
	lines := readableLines(body)
	keyValues := extractKeyValues(lines)
	sections := extractSections(lines)
	images := extractImages(doc, body)
	tags := extractTags(doc)

	transcript := domain.Transcript{
		Source:     "f95zone",
		SourceURL:  rawURL,
		ExternalID: InferExternalID(rawURL),
		Title:      title,
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

	doc.Find("meta[property='og:image']").Each(func(_ int, s *goquery.Selection) {
		add(s.AttrOr("content", ""))
	})
	body.Find("img").Each(func(_ int, s *goquery.Selection) {
		add(firstNonEmpty(
			s.AttrOr("data-src", ""),
			s.AttrOr("data-url", ""),
			s.AttrOr("src", ""),
		))
	})
	if len(images) > 50 {
		return images[:50]
	}
	return images
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

func inferMeta(t domain.Transcript) domain.TranscriptInferred {
	inferred := domain.TranscriptInferred{
		GameTitle:   inferGameTitle(t.Title),
		Version:     firstKeyValue(t.KeyValues, "Version"),
		Developer:   firstKeyValue(t.KeyValues, "Developer", "Publisher"),
		Description: firstSection(t.Sections, "Overview", "Story", "Description"),
	}
	if inferred.Version == "" {
		inferred.Version = inferVersionFromTitle(t.Title)
	}
	if inferred.Developer == "" {
		inferred.Developer = inferDeveloperFromTitle(t.Title)
	}
	if len(t.Images) > 0 {
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
