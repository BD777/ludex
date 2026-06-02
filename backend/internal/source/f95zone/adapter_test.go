package f95zone

import (
	"strings"
	"testing"
)

func TestParseHTMLExtractsThreadMeta(t *testing.T) {
	html := `
	<html>
		<head>
			<meta property="og:title" content="Sample Game [v1.2] [Example Dev] | F95zone">
			<meta property="og:image" content="https://example.test/cover.jpg">
		</head>
		<body>
			<h1 class="p-title-value">Sample Game [v1.2] [Example Dev]</h1>
			<article class="message--post">
				<div class="bbWrapper">
					<a href="https://example.test/cover-full.jpg"><img class="bbImage" src="https://example.test/cover-thumb.jpg" /></a>
					Overview:
					A short summary.

					Developer: Example Dev
					Censored: No
					Version: v1.2
					OS: Windows, Linux
					Language: English
					Genre: Adventure
					<a href="https://example.test/screen-1.jpg"><img class="bbImage" src="https://example.test/screen-1-thumb.jpg" /></a>
				</div>
			</article>
			<div class="js-tagList"><a>2dcg</a><a>adventure</a></div>
		</body>
	</html>`

	transcript, err := ParseHTML("https://f95zone.to/threads/sample-game.123456/", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if transcript.ExternalID != "123456" {
		t.Fatalf("external id = %q", transcript.ExternalID)
	}
	if transcript.Inferred.GameTitle != "Sample Game" {
		t.Fatalf("game title = %q", transcript.Inferred.GameTitle)
	}
	if transcript.Inferred.Version != "v1.2" {
		t.Fatalf("version = %q", transcript.Inferred.Version)
	}
	if transcript.Inferred.Developer != "Example Dev" {
		t.Fatalf("developer = %q", transcript.Inferred.Developer)
	}
	if len(transcript.Images) != 2 {
		t.Fatalf("images = %#v", transcript.Images)
	}
	if transcript.Fields.GameName != "Sample Game" {
		t.Fatalf("field game name = %q", transcript.Fields.GameName)
	}
	if transcript.Fields.CoverImage != "https://example.test/cover-full.jpg" {
		t.Fatalf("cover image = %q", transcript.Fields.CoverImage)
	}
	if transcript.Fields.Censored == nil || *transcript.Fields.Censored {
		t.Fatalf("censored = %#v", transcript.Fields.Censored)
	}
	if strings.Join(transcript.Fields.OperatingSystems, ",") != "Windows,Linux" {
		t.Fatalf("operating systems = %#v", transcript.Fields.OperatingSystems)
	}
}

func TestParseHTMLExtractsDownloadLinks(t *testing.T) {
	html := `
	<html>
		<body>
			<h1 class="p-title-value">Intertwined [v0.15] [Nyx]</h1>
			<article class="message--post">
				<div class="bbWrapper">
					<b>DOWNLOAD</b><br />
					<b>Win/Linux</b>: <a href="https://buzz.test/win">BUZZHEAVIER</a> - <a href="https://data.test/win.zip">DATANODES</a><br />
					<b>Mac</b> (v0.14.1): <a href="https://mega.test/mac">MEGA</a><br />
					<b>Patches:</b> <a href="https://patch.test">Ignore me</a>
				</div>
			</article>
		</body>
	</html>`

	transcript, err := ParseHTML("https://f95zone.to/threads/intertwined.53676/", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	groups := transcript.Fields.DownloadGroups
	if len(groups) != 2 {
		t.Fatalf("download groups = %#v", groups)
	}
	if groups[0].Platform != "Win/Linux" {
		t.Fatalf("first platform = %q", groups[0].Platform)
	}
	if len(groups[0].Links) != 2 {
		t.Fatalf("first links = %#v", groups[0].Links)
	}
	if groups[0].Links[0].Name != "BUZZHEAVIER" || groups[0].Links[0].URL != "https://buzz.test/win" {
		t.Fatalf("first link = %#v", groups[0].Links[0])
	}
	if groups[0].Links[1].Name != "DATANODES" || groups[0].Links[1].URL != "https://data.test/win.zip" {
		t.Fatalf("second link = %#v", groups[0].Links[1])
	}
	if groups[1].Platform != "Mac" {
		t.Fatalf("second platform = %q", groups[1].Platform)
	}
	if groups[1].Note != "v0.14.1" {
		t.Fatalf("second note = %q", groups[1].Note)
	}
	if len(groups[1].Links) != 1 || groups[1].Links[0].URL != "https://mega.test/mac" {
		t.Fatalf("second links = %#v", groups[1].Links)
	}
}

func TestParseHTMLExtractsSanitizedChangelogHTML(t *testing.T) {
	html := `
	<html>
		<body>
			<h1 class="p-title-value">Sample Game [v1.2] [Example Dev]</h1>
			<article class="message--post">
				<div class="bbWrapper">
					<b>Changelog</b>:<br />
					<div class="bbCodeSpoiler">
						<button type="button" data-xf-click="toggle">Spoiler</button>
						<div class="bbCodeSpoiler-content">
							<div class="bbCodeBlock-content">
								<b>v1.2</b><br />
								- Fixed a route<br />
								<ul><li data-xf-list-type="ul">Added a scene</li></ul>
								<script>alert("nope")</script>
							</div>
						</div>
					</div>
					<b>DOWNLOAD</b><br />
					Win/Linux: <a href="https://example.test/win">HOST</a>
				</div>
			</article>
		</body>
	</html>`

	transcript, err := ParseHTML("https://f95zone.to/threads/sample-game.123456/", strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(transcript.Fields.ChangelogHTML, "<strong>v1.2</strong><br>") {
		t.Fatalf("changelog html did not preserve bold line: %q", transcript.Fields.ChangelogHTML)
	}
	if !strings.Contains(transcript.Fields.ChangelogHTML, "<ul><li>Added a scene</li></ul>") {
		t.Fatalf("changelog html did not preserve list: %q", transcript.Fields.ChangelogHTML)
	}
	if strings.Contains(transcript.Fields.ChangelogHTML, "button") ||
		strings.Contains(transcript.Fields.ChangelogHTML, "data-xf") ||
		strings.Contains(transcript.Fields.ChangelogHTML, "script") ||
		strings.Contains(transcript.Fields.ChangelogHTML, "DOWNLOAD") {
		t.Fatalf("changelog html was not sanitized or bounded: %q", transcript.Fields.ChangelogHTML)
	}
}
