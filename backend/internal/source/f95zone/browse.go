package f95zone

import (
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"local/ludex/internal/domain"
)

const baseURL = "https://f95zone.to"

var filterCountRE = regexp.MustCompile(`^(.*?)\s*\(([^)]*)\)\s*$`)

func ParseBrowsePage(rawURL string, r io.Reader) (domain.AdapterBrowsePage, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return domain.AdapterBrowsePage{}, err
	}
	page := domain.AdapterBrowsePage{
		AdapterID: "f95zone",
		Title:     browseTitle(doc),
		URL:       rawURL,
		Page:      browseCurrentPage(doc),
		Items:     []domain.AdapterListItem{},
		Filters:   browseFilters(rawURL, doc),
	}
	page.TotalPages = browseTotalPages(doc)
	page.PrevURL = absoluteURL(rawURL, doc.Find(".pageNav-jump--prev, .pageNavSimple-el--prev").First().AttrOr("href", ""))
	page.NextURL = absoluteURL(rawURL, doc.Find(".pageNav-jump--next, .pageNavSimple-el--next").First().AttrOr("href", ""))

	doc.Find(".structItem--thread").Each(func(_ int, sel *goquery.Selection) {
		item := browseItem(rawURL, sel)
		if item.URL == "" || item.Title == "" || !item.Importable {
			return
		}
		page.Items = append(page.Items, item)
	})
	if len(page.Items) == 0 {
		page.Warnings = append(page.Warnings, "No importable F95zone threads were found on this list page.")
	}
	if page.Page <= 0 {
		page.Page = pageNumberFromURL(rawURL)
	}
	if page.Page <= 0 {
		page.Page = 1
	}
	return page, nil
}

func PresetListURL(presetID string, page int) string {
	if page <= 1 {
		page = 1
	}
	switch presetID {
	case "games":
		if page == 1 {
			return baseURL + "/forums/games.2/"
		}
		return baseURL + "/forums/games.2/page-" + strconv.Itoa(page)
	default:
		if page == 1 {
			return baseURL + "/trending/threads.1/"
		}
		return baseURL + "/trending/threads.1/?page=" + strconv.Itoa(page)
	}
}

func ListURLWithPage(rawURL string, page int) string {
	if page <= 1 {
		page = 1
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.Scheme == "" {
		base, _ := url.Parse(baseURL)
		parsed = base.ResolveReference(parsed)
	}
	query := parsed.Query()
	if query.Has("page") || strings.Contains(parsed.Path, "/trending/") {
		if page <= 1 {
			query.Del("page")
		} else {
			query.Set("page", strconv.Itoa(page))
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	parsed.Path = regexp.MustCompile(`/page-\d+/?$`).ReplaceAllString(parsed.Path, "/")
	if page > 1 {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/page-" + strconv.Itoa(page)
	}
	return parsed.String()
}

func browseTitle(doc *goquery.Document) string {
	title := cleanText(doc.Find("h1.p-title-value").First().Text())
	if title != "" {
		return title
	}
	title = cleanText(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	title = strings.TrimSuffix(title, " | F95zone")
	return title
}

func browseItem(rawURL string, sel *goquery.Selection) domain.AdapterListItem {
	if sel.Find(".structItem-status--sticky").Length() > 0 {
		return domain.AdapterListItem{}
	}
	link := sel.Find(".structItem-title a[data-tp-primary='on'][href*='/threads/']").First()
	if link.Length() == 0 {
		link = sel.Find(".structItem-title a[href*='/threads/']").Last()
	}
	threadURL := absoluteURL(rawURL, link.AttrOr("href", ""))
	title := cleanText(link.Text())
	prefixes := []string{}
	sel.Find(".structItem-title .labelLink").Each(func(_ int, label *goquery.Selection) {
		if text := cleanText(label.Text()); text != "" {
			prefixes = append(prefixes, text)
		}
	})

	item := domain.AdapterListItem{
		AdapterID:  "f95zone",
		ExternalID: InferExternalID(threadURL),
		Title:      stripKnownPrefixes(title, prefixes),
		URL:        threadURL,
		Author:     cleanText(firstNonEmpty(sel.AttrOr("data-author", ""), sel.Find(".structItem-parts .username").First().Text())),
		StartedAt:  sel.Find(".structItem-startDate time").First().AttrOr("datetime", ""),
		LatestAt:   sel.Find(".structItem-latestDate").First().AttrOr("datetime", ""),
		LatestBy:   cleanText(sel.Find(".structItem-cell--latest .username").First().Text()),
		Prefixes:   uniqueStrings(prefixes),
		Tags:       uniqueStrings(prefixes),
		Replies:    browsePair(sel, "Replies"),
		Views:      browsePair(sel, "Views"),
		Rating:     browseRating(sel),
		Votes:      browseVotes(sel),
		Importable: true,
	}
	if item.ExternalID == "" {
		item.Importable = false
	}
	return item
}

func browsePair(sel *goquery.Selection, key string) string {
	value := ""
	sel.Find(".structItem-cell--meta dl").EachWithBreak(func(_ int, pair *goquery.Selection) bool {
		if strings.EqualFold(cleanText(pair.Find("dt").Text()), key) {
			value = cleanText(pair.Find("dd").Text())
			return false
		}
		return true
	})
	return value
}

func browseRating(sel *goquery.Selection) string {
	title := cleanText(sel.Find(".ratingStars").First().AttrOr("title", ""))
	return strings.TrimSuffix(title, " star(s)")
}

func browseVotes(sel *goquery.Selection) string {
	text := cleanText(sel.Find(".ratingStarsRow-text").First().Text())
	return strings.TrimSuffix(text, " Votes")
}

func browseFilters(rawURL string, doc *goquery.Document) []domain.AdapterBrowseFilter {
	filters := []domain.AdapterBrowseFilter{}
	seen := map[string]bool{}
	doc.Find(".filterBar-prefix").Each(func(_ int, sel *goquery.Selection) {
		href := absoluteURL(rawURL, sel.AttrOr("href", ""))
		label := cleanText(sel.Text())
		count := ""
		if match := filterCountRE.FindStringSubmatch(label); len(match) == 3 {
			label = cleanText(match[1])
			count = cleanText(match[2])
		}
		if href == "" || label == "" || seen[href] {
			return
		}
		seen[href] = true
		filters = append(filters, domain.AdapterBrowseFilter{
			ID:    href,
			Label: label,
			Count: count,
			URL:   href,
		})
	})
	return filters
}

func browseCurrentPage(doc *goquery.Document) int {
	text := cleanText(doc.Find(".pageNav-page--current a, .pageNavSimple-el--current").First().Text())
	page, _ := strconv.Atoi(text)
	return page
}

func browseTotalPages(doc *goquery.Document) int {
	maxPage := 0
	doc.Find(".pageNav-page a, .pageNavSimple-el--last").Each(func(_ int, sel *goquery.Selection) {
		if page, err := strconv.Atoi(cleanText(sel.Text())); err == nil && page > maxPage {
			maxPage = page
			return
		}
		if page := pageNumberFromURL(sel.AttrOr("href", "")); page > maxPage {
			maxPage = page
		}
	})
	return maxPage
}

func pageNumberFromURL(rawURL string) int {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	if value := parsed.Query().Get("page"); value != "" {
		page, _ := strconv.Atoi(value)
		return page
	}
	match := regexp.MustCompile(`/page-(\d+)/?`).FindStringSubmatch(parsed.Path)
	if len(match) == 2 {
		page, _ := strconv.Atoi(match[1])
		return page
	}
	return 1
}

func absoluteURL(rawURL string, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(rawURL)
	if err != nil || base.Scheme == "" {
		base, _ = url.Parse(baseURL)
	}
	return base.ResolveReference(parsed).String()
}
