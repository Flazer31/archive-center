package httpapi

import "testing"

func Test46BodyHistoryReviewEndedCurrentKeepsOccurrencePregnancy(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("initial-success", "1423-01-14"))
	initialHistory := bodyHistory46(st)[0].EvidenceJSON
	if model := bodyModelRecord46(t, bodyHistory46(st)[0]); model["outcome"] != "latent_model_implantation" {
		t.Fatalf("guaranteed model fixture did not select an initial candidate: %#v", model)
	}
	saveBodyEvent46(t, srv, 2, bodyEvent46("pregnancy_ended", "known-ending", "1423-02-01"))
	ended := bodyCurrent46(t, srv).ValueJSON
	if parseJSONMap(ended)["modeled_pregnancy"] != nil || mapFromAny(parseJSONMap(ended)["pregnancy"])["status"] != "ended" {
		t.Fatal("observed ending did not clear the current model")
	}
	// One queried exposure is after ovulation but before any possible implantation;
	// the other is after every supported implantation day and before the ending.
	for i, date := range []string{"1423-01-16", "1423-01-30"} {
		exposure := bodyExposure46("historical-exposure-"+date, date)
		exposure["scene_scope"] = "flashback"
		saveBodyEvent46(t, srv, 3+i, exposure)
		event := bodyHistory46(st)[2+i]
		model := bodyModelRecord46(t, event)
		if model["status"] != "not_applicable" || model["reason"] != "modeled_pregnancy_ongoing" ||
			len(sliceFromAny(model["cycles"])) != 0 || len(sliceFromAny(parseJSONMap(event.EvidenceJSON)["model_cycles"])) != 0 {
			t.Fatalf("historical ongoing interval received a new trial: %#v", model)
		}
		if event.EventState != "history_only" || bodyCurrent46(t, srv).ValueJSON != ended {
			t.Fatal("historical model result changed the authoritative ended projection")
		}
	}
	// A genuinely later event is eligible after the known ending. With fixed
	// 28-day cycles this is O-1 in the next cycle, not the original pregnancy.
	saveBodyEvent46(t, srv, 5, bodyExposure46("post-ending-exposure", "1423-02-11"))
	newEvent := bodyHistory46(st)[4]
	newModel := bodyModelRecord46(t, newEvent)
	if newModel["status"] != "evaluated" || newModel["outcome"] != "latent_model_implantation" || len(sliceFromAny(parseJSONMap(newEvent.EvidenceJSON)["model_cycles"])) == 0 {
		t.Fatalf("known ending blocked a distinct later eligible cycle: %#v", newModel)
	}
	if bodyHistory46(st)[0].EvidenceJSON != initialHistory || len(bodyHistory46(st)) != 5 {
		t.Fatal("new observations rewrote the original model evidence or dropped history")
	}
}

func Test46BodyHistoryReviewUnknownEndingDoesNotInventAbsence(t *testing.T) {
	srv, st, _ := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("initial-success", "1423-01-14"))
	saveBodyEvent46(t, srv, 2, bodyEvent46("pregnancy_ended", "undated-ending", ""))
	before := bodyCurrent46(t, srv).ValueJSON
	ending := mapFromAny(parseJSONMap(before)["pregnancy"])
	if ending["status"] != "ended" || len(mapFromAny(ending["occurred_at"])) != 0 || parseJSONMap(before)["modeled_pregnancy"] != nil {
		t.Fatalf("unknown-date ending did not preserve its explicit meaning: %#v", ending)
	}
	exposure := bodyExposure46("historical-date-cannot-place-end", "1423-02-11")
	exposure["scene_scope"] = "flashback"
	saveBodyEvent46(t, srv, 3, exposure)
	event := bodyHistory46(st)[2]
	model := bodyModelRecord46(t, event)
	if model["status"] == "evaluated" || len(sliceFromAny(parseJSONMap(event.EvidenceJSON)["model_cycles"])) != 0 {
		t.Fatalf("unknown ending was treated as proof that this date was after pregnancy: %#v", model)
	}
	if event.EventState != "history_only" || bodyCurrent46(t, srv).ValueJSON != before || len(bodyHistory46(st)) != 3 {
		t.Fatal("uncertain historical interval erased current facts or lost the source observation")
	}
}

func Test46BodyHistoryReviewDisabledObservationStaysDisabledOnReplay(t *testing.T) {
	srv, st, cfg := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("initial-success", "1423-01-14"))
	selectedBefore := mustCompactJSON(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"])
	firstEvidence := bodyHistory46(st)[0].EvidenceJSON
	cfg.AutomaticPregnancyEnabled, cfg.CycleTrackingEnabled = false, true
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	exposure := bodyExposure46("while-automatic-disabled", "1423-02-11")
	saveBodyEvent46(t, srv, 2, exposure)
	event := bodyHistory46(st)[1]
	model := bodyModelRecord46(t, event)
	if model["status"] != "disabled" || len(sliceFromAny(parseJSONMap(event.EvidenceJSON)["model_cycles"])) != 0 ||
		mustCompactJSON(parseJSONMap(bodyCurrent46(t, srv).ValueJSON)["modeled_pregnancy"]) != selectedBefore {
		t.Fatalf("disabled observation drew again or replaced saved model state: %#v", model)
	}
	afterDisabled := bodyCurrent46(t, srv).ValueJSON
	cfg.AutomaticPregnancyEnabled = true
	if _, err := srv.saveBodyTrackingConfig("body-session", cfg); err != nil {
		t.Fatal(err)
	}
	saveBodyEvent46(t, srv, 3, exposure)
	if len(bodyHistory46(st)) != 2 || bodyCurrent46(t, srv).ValueJSON != afterDisabled || bodyHistory46(st)[0].EvidenceJSON != firstEvidence {
		t.Fatal("re-enabling retroactively sampled a recorded disabled event or changed history")
	}
}
