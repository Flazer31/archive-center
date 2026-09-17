package httpapi

func buildPostChromaTop1ScopedVerbatim() map[string]any {
	return map[string]any{
		"version":          "seq18_p327.v1",
		"role":             "post_chroma_top1_scoped_verbatim",
		"truth_authority":  false,
		"top":              1,
		"lane":             "scoped_verbatim_recall",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop2HybridScoring defines the Post-Chroma Top 2 summary
// surface for SEQ-18-P328: hybrid retrieval scoring baseline evidence.
func buildPostChromaTop2HybridScoring() map[string]any {
	return map[string]any{
		"version":          "seq18_p328.v1",
		"role":             "post_chroma_top2_hybrid_scoring",
		"truth_authority":  false,
		"top":              2,
		"lane":             "hybrid_retrieval_scoring_baseline",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "hy1a.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop3TemporalRelation defines the Post-Chroma Top 3 summary
// surface for SEQ-18-P329: temporal relation + story clock foundation evidence.
func buildPostChromaTop3TemporalRelation() map[string]any {
	return map[string]any{
		"version":          "seq18_p329.v1",
		"role":             "post_chroma_top3_temporal_relation",
		"truth_authority":  false,
		"top":              3,
		"lane":             "temporal_relation_story_clock",
		"evidence_surface": "qr_temporal_profile_budget",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop4TemporalValidity defines the Post-Chroma Top 4 summary
// surface for SEQ-18-P330: temporal validity retrieval evidence.
func buildPostChromaTop4TemporalValidity() map[string]any {
	return map[string]any{
		"version":          "seq18_p330.v1",
		"role":             "post_chroma_top4_temporal_validity",
		"truth_authority":  false,
		"top":              4,
		"lane":             "temporal_validity_retrieval",
		"evidence_surface": "qr_temporal_profile_budget",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop5EntityGraph defines the Post-Chroma Top 5 summary
// surface for SEQ-18-P331: lightweight entity / graph retrieval accelerator evidence.
func buildPostChromaTop5EntityGraph() map[string]any {
	return map[string]any{
		"version":          "seq18_p331.v1",
		"role":             "post_chroma_top5_entity_graph",
		"truth_authority":  false,
		"top":              5,
		"lane":             "lightweight_entity_graph_accelerator",
		"evidence_surface": "post_chroma_top5_entity_graph",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop6SelectiveRerank defines the Post-Chroma Top 6 summary
// surface for SEQ-18-P332: selective rerank + budget-aware routing evidence.
func buildPostChromaTop6SelectiveRerank() map[string]any {
	return map[string]any{
		"version":          "seq18_p332.v1",
		"role":             "post_chroma_top6_selective_rerank",
		"truth_authority":  false,
		"top":              6,
		"lane":             "selective_rerank_budget_aware_routing",
		"evidence_surface": "hy_tail_budget_policy_owner",
		"policy_version":   "hy1d.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildVRRawPreservingSupport defines the VR raw-preserving support summary
// surface for SEQ-18-P336: verbatim recall support lane.
func buildVRRawPreservingSupport() map[string]any {
	return map[string]any{
		"version":          "seq18_p336.v1",
		"role":             "vr_raw_preserving_support",
		"truth_authority":  false,
		"support_lane":     "verbatim_recall",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRHybridRealism defines the VR hybrid realism summary surface for
// SEQ-18-P337: semantic-only keyword/bias evidence.
func buildVRHybridRealism() map[string]any {
	return map[string]any{
		"version":          "seq18_p337.v1",
		"role":             "vr_hybrid_realism",
		"truth_authority":  false,
		"aspect":           "semantic_only_keyword_bias",
		"evidence_surface": "hy_soft_bias",
		"policy_version":   "hy1a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRSoftRouting defines the VR soft routing summary surface for
// SEQ-18-P338: query class evidence.
func buildVRSoftRouting() map[string]any {
	return map[string]any{
		"version":          "seq18_p338.v1",
		"role":             "vr_soft_routing",
		"truth_authority":  false,
		"aspect":           "query_class",
		"evidence_surface": "qr_query_class_contract",
		"policy_version":   "qr1a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRLatencyDiscipline defines the VR latency discipline summary surface
// for SEQ-18-P339: recall budget evidence.
func buildVRLatencyDiscipline() map[string]any {
	return map[string]any{
		"version":          "seq18_p339.v1",
		"role":             "vr_latency_discipline",
		"truth_authority":  false,
		"aspect":           "recall_budget",
		"evidence_surface": "qr_query_class_budget_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRTruthBoundaryPreserve defines the VR truth-boundary preserve summary
// surface for SEQ-18-P340: Chroma hit canonical authority evidence.
func buildVRTruthBoundaryPreserve() map[string]any {
	return map[string]any{
		"version":          "seq18_p340.v1",
		"role":             "vr_truth_boundary_preserve",
		"truth_authority":  false,
		"aspect":           "chroma_hit_canonical_authority",
		"evidence_surface": "vx_truth_boundary_gate",
		"policy_version":   "vx18d.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVR18_1aRawTranscript defines the VR 18-1a raw transcript / direct-evidence
// support lane summary surface for SEQ-18-P344.
func buildVR18_1aRawTranscript() map[string]any {
	return map[string]any{
		"version":          "seq18_p344.v1",
		"role":             "vr_18_1a_raw_transcript",
		"truth_authority":  false,
		"sub_step":         "18-1a",
		"lane":             "raw_transcript_direct_evidence",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1bSourceTag defines the VR 18-1b source-tag / scope metadata /
// snippet policy summary surface for SEQ-18-P345.
func buildVR18_1bSourceTag() map[string]any {
	return map[string]any{
		"version":          "seq18_p345.v1",
		"role":             "vr_18_1b_source_tag",
		"truth_authority":  false,
		"sub_step":         "18-1b",
		"aspect":           "source_tag_scope_metadata_snippet_policy",
		"evidence_surface": "vr_policy_owner_block",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1cPromptInjection defines the VR 18-1c prompt injection support
// surface summary for SEQ-18-P346.
func buildVR18_1cPromptInjection() map[string]any {
	return map[string]any{
		"version":          "seq18_p346.v1",
		"role":             "vr_18_1c_prompt_injection",
		"truth_authority":  false,
		"sub_step":         "18-1c",
		"aspect":           "prompt_injection_support_surface",
		"evidence_surface": "vr_prompt_injection_strategy",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1dHierarchyEscape defines the VR 18-1d hierarchy escape hatch
// summary surface for SEQ-18-P347: dense summary miss raw/direct-evidence slice.
func buildVR18_1dHierarchyEscape() map[string]any {
	return map[string]any{
		"version":          "seq18_p347.v1",
		"role":             "vr_18_1d_hierarchy_escape",
		"truth_authority":  false,
		"sub_step":         "18-1d",
		"aspect":           "hierarchy_escape_hatch",
		"evidence_surface": "vr_hierarchy_escape_hatch",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildHY18_2aSemanticKeyword defines the HY 18-2a semantic + keyword baseline
// score summary surface for SEQ-18-P351.
func buildHY18_2aSemanticKeyword() map[string]any {
	return map[string]any{
		"version":          "seq18_p351.v1",
		"role":             "hy_18_2a_semantic_keyword",
		"truth_authority":  false,
		"sub_step":         "18-2a",
		"aspect":           "semantic_keyword_baseline_score",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2bSoftBias defines the HY 18-2b speaker/location/storyline soft
// bias summary surface for SEQ-18-P352.
func buildHY18_2bSoftBias() map[string]any {
	return map[string]any{
		"version":          "seq18_p352.v1",
		"role":             "hy_18_2b_soft_bias",
		"truth_authority":  false,
		"sub_step":         "18-2b",
		"aspect":           "speaker_location_storyline_soft_bias",
		"evidence_surface": "hy_soft_bias",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2cScoreInspection defines the HY 18-2c hybrid score inspection
// surface summary for SEQ-18-P353.
func buildHY18_2cScoreInspection() map[string]any {
	return map[string]any{
		"version":          "seq18_p353.v1",
		"role":             "hy_18_2c_score_inspection",
		"truth_authority":  false,
		"sub_step":         "18-2c",
		"aspect":           "hybrid_score_inspection_surface",
		"evidence_surface": "hy_runtime_inspection",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2dAdaptiveTopK defines the HY 18-2d adaptive top-k / tail-budget
// summary surface for SEQ-18-P354: budget near-cutoff bounded tail rescue promotion.
func buildHY18_2dAdaptiveTopK() map[string]any {
	return map[string]any{
		"version":          "seq18_p354.v1",
		"role":             "hy_18_2d_adaptive_topk",
		"truth_authority":  false,
		"sub_step":         "18-2d",
		"aspect":           "adaptive_topk_tail_budget",
		"evidence_surface": "hy_tail_budget_rescue_pass",
		"policy_version":   "hy1d.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildQR18_3aQueryClass defines the QR 18-3a callback/resume/canon/scene/
// temporal query class summary surface for SEQ-18-P358.
func buildQR18_3aQueryClass() map[string]any {
	return map[string]any{
		"version":          "seq18_p358.v1",
		"role":             "qr_18_3a_query_class",
		"truth_authority":  false,
		"sub_step":         "18-3a",
		"aspect":           "callback_resume_canon_scene_temporal_query_class",
		"evidence_surface": "qr_query_class_taxonomy",
		"policy_version":   "qr1a.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3bRetrievalDepth defines the QR 18-3b query class retrieval depth /
// candidate budget summary surface for SEQ-18-P359.
func buildQR18_3bRetrievalDepth() map[string]any {
	return map[string]any{
		"version":          "seq18_p359.v1",
		"role":             "qr_18_3b_retrieval_depth",
		"truth_authority":  false,
		"sub_step":         "18-3b",
		"aspect":           "query_class_retrieval_depth_candidate_budget",
		"evidence_surface": "qr_budget_visibility",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3cExtractBeforeRead defines the QR 18-3c extract-before-read
// retrieval note surface summary for SEQ-18-P360.
func buildQR18_3cExtractBeforeRead() map[string]any {
	return map[string]any{
		"version":          "seq18_p360.v1",
		"role":             "qr_18_3c_extract_before_read",
		"truth_authority":  false,
		"sub_step":         "18-3c",
		"aspect":           "extract_before_read_retrieval_note_surface",
		"evidence_surface": "qr_query_class_budget_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3dLongTailRoute defines the QR 18-3d callback / needle-in-haystack /
// old-detail query route summary surface for SEQ-18-P361: long-tail miss scene recall split.
func buildQR18_3dLongTailRoute() map[string]any {
	return map[string]any{
		"version":          "seq18_p361.v1",
		"role":             "qr_18_3d_long_tail_route",
		"truth_authority":  false,
		"sub_step":         "18-3d",
		"aspect":           "callback_needle_in_haystack_old_detail_query_route",
		"evidence_surface": "qr_note_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildVX18_4aSemanticHybridReplay defines the VX 18-4a semantic-only vs
// hybrid replay summary surface for SEQ-18-P365.
func buildVX18_4aSemanticHybridReplay() map[string]any {
	return map[string]any{
		"version":          "seq18_p365.v1",
		"role":             "vx_18_4a_semantic_hybrid_replay",
		"truth_authority":  false,
		"sub_step":         "18-4a",
		"aspect":           "semantic_only_vs_hybrid_replay",
		"evidence_surface": "vx_hybrid_replay_gate",
		"policy_version":   "vx18a.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4bHeldOutRecall defines the VX 18-4b held-out recall completeness
// gate summary surface for SEQ-18-P366.
func buildVX18_4bHeldOutRecall() map[string]any {
	return map[string]any{
		"version":          "seq18_p366.v1",
		"role":             "vx_18_4b_held_out_recall",
		"truth_authority":  false,
		"sub_step":         "18-4b",
		"aspect":           "held_out_recall_completeness_gate",
		"evidence_surface": "vx_heldout_completeness_gate",
		"policy_version":   "vx18b.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4cLatencyToken defines the VX 18-4c latency / token budget gate
// summary surface for SEQ-18-P367.
func buildVX18_4cLatencyToken() map[string]any {
	return map[string]any{
		"version":          "seq18_p367.v1",
		"role":             "vx_18_4c_latency_token",
		"truth_authority":  false,
		"sub_step":         "18-4c",
		"aspect":           "latency_token_budget_gate",
		"evidence_surface": "vx_latency_token_budget_gate",
		"policy_version":   "vx18c.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4dTruthBoundaryReplay defines the VX 18-4d truth-boundary replay
// summary surface for SEQ-18-P368.
func buildVX18_4dTruthBoundaryReplay() map[string]any {
	return map[string]any{
		"version":          "seq18_p368.v1",
		"role":             "vx_18_4d_truth_boundary_replay",
		"truth_authority":  false,
		"sub_step":         "18-4d",
		"aspect":           "truth_boundary_replay",
		"evidence_surface": "vx_truth_boundary_gate",
		"policy_version":   "vx18d.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4eTopKTruncation defines the VX 18-4e top-k truncation / summary-loss
// regression gate summary surface for SEQ-18-P369: tail fact miss actual held-out verify.
func buildVX18_4eTopKTruncation() map[string]any {
	return map[string]any{
		"version":          "seq18_p369.v1",
		"role":             "vx_18_4e_topk_truncation",
		"truth_authority":  false,
		"sub_step":         "18-4e",
		"aspect":           "topk_truncation_summary_loss_regression_gate",
		"evidence_surface": "vx_truncation_summary_loss_gate",
		"policy_version":   "vx18e.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildPreReleaseVersionMarker defines the pre-release version marker
// summary surface for SEQ-18-P373: root runtime version marker file 1.0.0-pre promotion.
func buildPreReleaseVersionMarker() map[string]any {
	return map[string]any{
		"version":         "seq18_p373.v1",
		"role":            "pre_release_version_marker",
		"truth_authority": false,
		"marker_file":     "1.0.0-pre",
		"promotion":       true,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_summary_surface",
	}
}

// buildPreReleaseBundleAuthority defines the pre-release bundle authority
// summary surface for SEQ-18-P374: Archive Center Pre-release 1.0.0 bundle latest
// current working runtime authority create/generate.
func buildPreReleaseBundleAuthority() map[string]any {
	return map[string]any{
		"version":          "seq18_p374.v1",
		"role":             "pre_release_bundle_authority",
		"truth_authority":  false,
		"bundle_name":      "Archive Center Pre-release 1.0.0",
		"authority":        "latest_current_working_runtime",
		"evidence_surface": "vr_regression_bundle_green",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_summary_surface",
	}
}

// buildPreReleaseArtifact defines the pre-release artifact summary surface
// for SEQ-18-P375: latest validated bundle artifact.
func buildPreReleaseArtifact() map[string]any {
	return map[string]any{
		"version":         "seq18_p375.v1",
		"role":            "pre_release_artifact",
		"truth_authority": false,
		"artifact_name":   "Archive Center Pre-release 1.0.0",
		"validated":       true,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_summary_surface",
	}
}

// buildPreReleaseVRSmoke defines the pre-release VR smoke check summary
// surface for SEQ-18-P376: scoped verbatim recall smoke check pass.
func buildPreReleaseVRSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p376.v1",
		"role":             "pre_release_vr_smoke",
		"truth_authority":  false,
		"check":            "scoped_verbatim_recall_smoke",
		"status":           "pass",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseHYSmoke defines the pre-release HY smoke check summary
// surface for SEQ-18-P377: hybrid baseline smoke check pass.
func buildPreReleaseHYSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p377.v1",
		"role":             "pre_release_hy_smoke",
		"truth_authority":  false,
		"check":            "hybrid_baseline_smoke",
		"status":           "pass",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseQRSmoke defines the pre-release QR smoke check summary
// surface for SEQ-18-P378: query-class routing / budget smoke check pass.
func buildPreReleaseQRSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p378.v1",
		"role":             "pre_release_qr_smoke",
		"truth_authority":  false,
		"check":            "query_class_routing_budget_smoke",
		"status":           "pass",
		"evidence_surface": "qr_query_class_contract",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseVXReview defines the pre-release VX review checklist summary
// surface for SEQ-18-P379: held-out / latency / truth-boundary / truncation-summary-loss
// review checklist pass.
func buildPreReleaseVXReview() map[string]any {
	return map[string]any{
		"version":         "seq18_p379.v1",
		"role":            "pre_release_vx_review",
		"truth_authority": false,
		"check":           "vx_review_checklist",
		"status":          "pass",
		"components": []string{
			"held_out_completeness",
			"latency_token_budget",
			"truth_boundary_replay",
			"truncation_summary_loss",
		},
		"policy_version": "pr1a.v1",
		"mode":           "pre_release_review_summary_surface",
	}
}

// buildPreReleaseRawSnippet defines the pre-release raw support snippet
// summary surface for SEQ-18-P391: raw support snippet 3 / 720 chars / excerpt 160 chars.
func buildPreReleaseRawSnippet() map[string]any {
	return map[string]any{
		"version":         "seq18_p391.v1",
		"role":            "pre_release_raw_snippet",
		"truth_authority": false,
		"snippet_count":   3,
		"max_chars":       720,
		"excerpt_chars":   160,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseHybridBias defines the pre-release hybrid score soft bias
// summary surface for SEQ-18-P392: speaker 0.04 / location 0.05 / storyline 0.06 / cap 0.12.
func buildPreReleaseHybridBias() map[string]any {
	return map[string]any{
		"version":         "seq18_p392.v1",
		"role":            "pre_release_hybrid_bias",
		"truth_authority": false,
		"speaker":         0.04,
		"location":        0.05,
		"storyline":       0.06,
		"cap":             0.12,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseQueryClassRule defines the pre-release query class rule
// heuristic summary surface for SEQ-18-P393: rule-first additive contract + fail-open
// shared execution.
func buildPreReleaseQueryClassRule() map[string]any {
	return map[string]any{
		"version":         "seq18_p393.v1",
		"role":            "pre_release_query_class_rule",
		"truth_authority": false,
		"heuristic":       "rule_first_additive_contract",
		"execution":       "fail_open_shared_execution",
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseRetrievalNote defines the pre-release retrieval note /
// extract surface default summary surface for SEQ-18-P394: support_surface_first,
// scene/canon no-extract default, callback/resume/temporal note-only until route exec.
func buildPreReleaseRetrievalNote() map[string]any {
	return map[string]any{
		"version":         "seq18_p394.v1",
		"role":            "pre_release_retrieval_note",
		"truth_authority": false,
		"defaults": map[string]any{
			"support_surface_first":              true,
			"scene_canon_no_extract":             true,
			"callback_resume_temporal_note_only": true,
			"note_only_until_route_exec":         true,
		},
		"policy_version": "pr1a.v1",
		"mode":           "pre_release_detail_summary_surface",
	}
}
