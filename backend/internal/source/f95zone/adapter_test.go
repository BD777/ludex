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
