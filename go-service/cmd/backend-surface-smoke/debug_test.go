package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDebugSmoke(t *testing.T) {
	rep := runSurfaceSmoke("test-session-42")
	for _, c := range rep.Checks {
		fmt.Printf("phase=%s route=%s status=%s http=%d json=%v keys=%v notes=%q\n", c.Phase, c.Route, c.Status, c.HTTPStatus, c.JSONValid, c.TopLevelKeys, c.MissingNotes)
	}
	data, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(data))
}
