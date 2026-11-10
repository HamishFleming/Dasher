package mapsdata

import (
	"testing"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/page"
)

// Chromium 152 sends this reason on initial frame navigation. Older bindings
// reject the whole event; exercise the same decoder chromedp uses.
func TestInitialFrameNavigationEvent(t *testing.T) {
	event, err := cdproto.UnmarshalMessage(&cdproto.Message{
		Method: cdproto.EventPageFrameRequestedNavigation,
		Params: []byte(`{"frameId":"fixture-frame","reason":"initialFrameNavigation","url":"about:blank","disposition":"currentTab"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	navigation, ok := event.(*page.EventFrameRequestedNavigation)
	if !ok || string(navigation.Reason) != "initialFrameNavigation" {
		t.Fatalf("unexpected event: %#v", event)
	}
}
