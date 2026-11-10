package mapsdata

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/chromedp/chromedp"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// Opt-in browser smoke test uses an in-process HTTP recorder and a data URL:
// no HTTP listener, remote-debugging port, Google request or external resource.
func TestChromiumExtractionFixture(t *testing.T) {
	executable := os.Getenv("MAPSDATA_TEST_CHROMIUM")
	if executable == "" {
		t.Skip("set MAPSDATA_TEST_CHROMIUM to run the offline browser fixture")
	}
	for _, tc := range []struct {
		name, body  string
		blocked     bool
		busy, hours int
	}{
		{"observations", `<main role="main"><h1>Fixture Market</h1><section><h2>Popular times</h2><button aria-haspopup="listbox">Tuesday</button><div aria-label="50% busy at 12 pm."></div><div aria-label="Live: currently busy at 1 pm."></div><div hidden aria-label="90% busy at 2 pm."></div></section><table><tr><td>Tuesday</td><td>9 am–5 pm</td></tr><tr><td>Wednesday</td><td>Closed</td></tr></table></main>`, false, 1, 2},
		{"limited-hours-outside-main", `<h1>Fixture Market</h1><p>You're seeing a limited view of Google Maps.</p><table role="presentation"><tr><td><div>Tuesday</div></td><td aria-label="Open 24 hours"><ul><li>Open 24 hours</li></ul></td><td><button>Copy open hours</button></td></tr></table>`, false, 0, 1},
		{"full-week", weeklyFixture(), false, 0, 7},
		{"active-chart", `<h1>Fixture Market</h1><section><h2>Popular times</h2><button aria-haspopup="menu" aria-label="Tuesdays">Tuesdays</button><div role="img" aria-label="10% busy at 8 am."></div><div aria-hidden="true"><div role="img" aria-label="99% busy at 8 am."></div></div></section>`, false, 1, 0},
		{"blocked", `<h1>Consent</h1><form action="https://consent.google.com/save"></form>`, true, 0, 0},
		{"missing", `<main role="main"><h1>Fixture Market</h1><p>No observations</p></main>`, false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`<!doctype html><meta charset="utf-8">` + tc.body + `<script>document.body.insertAdjacentHTML('beforeend','<pre id="result"></pre>');document.getElementById('result').textContent=JSON.stringify(` + extractJS + `);</script>`))
			}).ServeHTTP(recorder, httptest.NewRequest("GET", "http://fixture.invalid", nil))
			target := "data:text/html;base64," + base64.StdEncoding.EncodeToString(recorder.Body.Bytes())
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "--headless", "--disable-gpu", "--disable-background-networking", "--no-first-run", "--no-default-browser-check", "--host-resolver-rules=MAP * ~NOTFOUND", "--user-data-dir="+t.TempDir(), "--dump-dom", target)
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("browser fixture: %v", err)
			}
			dom := string(output)
			start := strings.LastIndex(dom, `<pre id="result">`)
			if start < 0 {
				t.Fatal(dom)
			}
			start += len(`<pre id="result">`)
			end := strings.Index(dom[start:], "</pre>")
			if end < 0 {
				t.Fatal(dom)
			}
			var got pageSnapshot
			if err = json.Unmarshal([]byte(html.UnescapeString(dom[start:start+end])), &got); err != nil {
				t.Fatal(err)
			}
			if got.Blocked != tc.blocked || len(got.Busy) != tc.busy || len(got.Hours) != tc.hours {
				t.Fatalf("%+v", got)
			}
			if tc.name == "limited-hours-outside-main" && (!got.LimitedView || got.Hours[0].Text != "Open 24 hours") {
				t.Fatal(got)
			}
			if tc.name == "full-week" && (hoursText(got.Hours, "Monday") != "Closed" || hoursText(got.Hours, "Friday") != "7 am–12 pm, 1 pm–2:30 pm" || hoursText(got.Hours, "Thursday") != "7 am–2:30 pm") {
				t.Fatal(got)
			}
			if tc.name == "active-chart" && (got.Busy[0].Day != "Tuesday" || len(got.Busy[0].Labels) != 1 || got.Busy[0].Labels[0] != "10% busy at 8 am.") {
				t.Fatal(got)
			}
			if tc.name == "observations" && (got.Busy[0].Day != "Tuesday" || got.Busy[0].Labels[0] != "50% busy at 12 pm.") {
				t.Fatal(got)
			}
		})
	}
}

// Exercise the protocol transport as well as the DOM extractor. The new
// chromedp uses a debugging pipe, so this fixture opens no listening socket.
func TestChromiumExtractionFixtureProtocol(t *testing.T) {
	executable := os.Getenv("MAPSDATA_TEST_CHROMIUM")
	if executable == "" {
		t.Skip("set MAPSDATA_TEST_CHROMIUM to run the offline protocol fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(executable), chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND"))
	alloc, closeAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer closeAlloc()
	var mu sync.Mutex
	var protocolErrors []string
	browser, closeBrowser := chromedp.NewContext(alloc, chromedp.WithErrorf(func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		protocolErrors = append(protocolErrors, fmt.Sprintf(format, args...))
	}))
	defer closeBrowser()
	target := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(`<h1>Protocol fixture</h1><iframe srcdoc="<p>frame</p>"></iframe>`))
	var title string
	if err := chromedp.Do(browser, chromedp.Navigate(target), chromedp.Poll[chromedp.Void](`document.querySelector('iframe')?.contentDocument?.querySelector('p')`, chromedp.WithPollingTimeout(5*time.Second)), evaluateInto(`document.querySelector('h1').innerText`, &title)); err != nil {
		t.Fatal(err)
	}
	closeBrowser()
	if title != "Protocol fixture" {
		t.Fatal(title)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, message := range protocolErrors {
		if strings.Contains(message, "unmarshal") {
			t.Error(message)
		}
	}
}

func weeklyFixture() string {
	rows := ""
	for _, day := range weekdays {
		value := "<li>7 am–2:30 pm</li>"
		if day == "Monday" {
			value = "<li>Closed</li>"
		}
		if day == "Friday" {
			value = "<li>7 am–12 pm</li><li>1 pm–2:30 pm</li>"
		}
		style := ""
		if day == "Thursday" {
			style = ` style="display:none"`
		}
		rows += `<tr` + style + `><td><div>` + day + `</div></td><td><ul>` + value + `</ul></td><td><button>Copy open hours</button></td></tr>`
	}
	return `<h1>Fixture Market</h1><div class="t39EBf"><table role="presentation">` + rows + `</table></div><h2>Kitchen</h2><table><tr><td>Monday</td><td>5 pm–9 pm</td></tr></table>`
}

func TestChromiumExtractionFixtureDelayedWeek(t *testing.T) {
	executable := os.Getenv("MAPSDATA_TEST_CHROMIUM")
	if executable == "" {
		t.Skip("set MAPSDATA_TEST_CHROMIUM for offline browser fixtures")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(executable), chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND"))
	alloc, closeAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer closeAlloc()
	browser, closeBrowser := chromedp.NewContext(alloc)
	defer closeBrowser()
	// The inline summary action is intentionally the wrong control. The primary
	// button opens the full panel, whose other six days render asynchronously.
	full, _ := json.Marshal(weeklyFixture())
	body := `<h1>Fixture Market</h1><button aria-label="Show open hours for the week" onclick="throw Error('wrong summary control')">summary</button><button data-item-id="oh" aria-label="Closed · See more hours" onclick='document.body.innerHTML="<h1>Hours</h1><table><tr><td>Tuesday</td><td>7 am–2:30 pm</td></tr></table>";setTimeout(()=>document.body.innerHTML=` + string(full) + `,200)'>See more hours</button>`
	target := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(body))
	if err := chromedp.Do(browser, desktopPage(), chromedp.Navigate(target)); err != nil {
		t.Fatal(err)
	}
	expanded, err := expandWeeklyHours(browser)
	if err != nil || !expanded {
		t.Fatal(expanded, err)
	}
	ready := `(() => {const r=` + extractJS + `;return r.hours.length===7;})()`
	var result pageSnapshot
	if err = chromedp.Do(browser, waitForPage(ready, chromedp.WithPollingTimeout(3*time.Second)), evaluateInto(extractJS, &result)); err != nil {
		t.Fatal(err)
	}
	if len(result.Hours) != 7 || hoursText(result.Hours, "Monday") != "Closed" {
		t.Fatal(result)
	}
	agent, err := chromedp.Run(browser, chromedp.Evaluate[string](`navigator.userAgent`))
	if err != nil || strings.Contains(agent, "HeadlessChrome/") {
		t.Fatal(agent, err)
	}
}
