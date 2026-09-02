package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/pdfmemory"
)

const (
	prepareTurnMemoryTransportPlanContract    = "memory_transport_plan.v1"
	prepareTurnMemoryTransportPayloadContract = "memory_transport_payload.v1"
	prepareTurnMemoryTransportModeText        = "text"
	prepareTurnMemoryTransportModeGooglePDF   = "google_pdf"
	prepareTurnMemoryTransportModeGatewayPDF  = "llm_gateway_pdf"
	prepareTurnMemoryTransportModeProviderPDF = "provider_manager_pdf"
	prepareTurnMemoryPDFMIME                  = "application/pdf"
	prepareTurnMemoryPDFTokensPerPage         = 258
)

type prepareTurnMemoryPDFGenerator func(string) (pdfmemory.Document, error)

func normalizePrepareTurnMemoryTransportMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pdf", prepareTurnMemoryTransportModeGooglePDF:
		// "pdf" is the compatibility spelling used by the isolated experiment.
		return prepareTurnMemoryTransportModeGooglePDF
	case prepareTurnMemoryTransportModeGatewayPDF:
		return prepareTurnMemoryTransportModeGatewayPDF
	case prepareTurnMemoryTransportModeProviderPDF:
		return prepareTurnMemoryTransportModeProviderPDF
	default:
		return prepareTurnMemoryTransportModeText
	}
}

func prepareTurnMemoryTransportBodyFormat(mode string) string {
	switch mode {
	case prepareTurnMemoryTransportModeGooglePDF:
		return "gemini_inline_data"
	case prepareTurnMemoryTransportModeGatewayPDF:
		return "openai_file"
	case prepareTurnMemoryTransportModeProviderPDF:
		return "provider_manager_manual_pdf"
	default:
		return "text"
	}
}

func prepareTurnMemoryTransportAuxiliaryWithoutLongTermMemory(payloadPlan map[string]any) string {
	parts := []string{}
	for _, raw := range outputFidelityLineageSlice(payloadPlan["lanes"]) {
		lane := mapFromAny(raw)
		if extractionStringFromAny(lane["key"]) == "long_term_memory" || !boolFromAny(lane["applied"]) {
			continue
		}
		if text := extractionStringFromAny(lane["text"]); strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func prepareTurnMemoryTransportLongTermMemory(payloadPlan map[string]any) string {
	for _, raw := range outputFidelityLineageSlice(payloadPlan["lanes"]) {
		lane := mapFromAny(raw)
		if extractionStringFromAny(lane["key"]) != "long_term_memory" || !boolFromAny(lane["applied"]) {
			continue
		}
		return extractionStringFromAny(lane["text"])
	}
	return ""
}

func prepareTurnMemoryTransportPDFErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, pdfmemory.ErrEmptyText):
		return "memory_pdf_empty"
	case errors.Is(err, pdfmemory.ErrInvalidUTF8):
		return "memory_pdf_invalid_utf8"
	case errors.Is(err, pdfmemory.ErrUnsupportedText):
		return "memory_pdf_unsupported_text"
	case errors.Is(err, pdfmemory.ErrFontLoad):
		return "memory_pdf_font_load_failed"
	case errors.Is(err, pdfmemory.ErrRender):
		return "memory_pdf_render_failed"
	default:
		return "memory_pdf_build_failed"
	}
}

func prepareTurnMemoryTransportPDFHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func buildPrepareTurnMemoryTransport(
	requestedMode string,
	payloadPlan map[string]any,
	requestCorrelationID string,
	generate prepareTurnMemoryPDFGenerator,
) (map[string]any, map[string]any) {
	requestedMode = normalizePrepareTurnMemoryTransportMode(requestedMode)
	auxiliaryText := extractionStringFromAny(payloadPlan["auxiliary_text"])
	withoutLongTermMemory := prepareTurnMemoryTransportAuxiliaryWithoutLongTermMemory(payloadPlan)
	logicalMemory := prepareTurnMemoryTransportLongTermMemory(payloadPlan)
	logicalHash := prepareTurnTextHash(logicalMemory)
	logicalMemoryChars := len([]rune(logicalMemory))
	estimatedTextTokens := (logicalMemoryChars + 1) / 2

	plan := map[string]any{
		"contract_version":                   prepareTurnMemoryTransportPlanContract,
		"owner":                              "go",
		"scope":                              "current_request_only",
		"target_lane":                        "long_term_memory",
		"requested_mode":                     requestedMode,
		"selected_mode":                      prepareTurnMemoryTransportModeText,
		"body_format":                        "text",
		"transport_status":                   "text_mode",
		"generation_status":                  "not_requested",
		"error_code":                         nil,
		"logical_memory_chars":               logicalMemoryChars,
		"long_term_memory_text":              logicalMemory,
		"logical_text_hash":                  logicalHash,
		"pdf_bytes_hash":                     "",
		"filename":                           "",
		"mime_type":                          "",
		"page_count":                         0,
		"pdf_bytes":                          0,
		"base64_chars":                       0,
		"estimated_text_tokens":              estimatedTextTokens,
		"estimated_pdf_tokens":               nil,
		"estimated_token_delta":              nil,
		"estimated_token_delta_percent":      nil,
		"token_estimate_status":              "unavailable",
		"token_estimate_basis":               "logical_chars_div_2_vs_gemini_pdf_pages_258",
		"auxiliary_text":                     auxiliaryText,
		"auxiliary_text_hash":                prepareTurnTextHash(auxiliaryText),
		"auxiliary_without_long_term_memory": withoutLongTermMemory,
		"auxiliary_without_long_term_memory_hash": prepareTurnTextHash(withoutLongTermMemory),
		"text_baseline_retained":                  true,
		"transient_payload_included":              false,
		"transient_payload_available":             false,
	}

	var transient map[string]any
	if requestedMode == prepareTurnMemoryTransportModeProviderPDF {
		if logicalMemory == "" {
			plan["generation_status"] = "empty"
			plan["error_code"] = "memory_pdf_empty"
		} else {
			plan["selected_mode"] = requestedMode
			plan["body_format"] = prepareTurnMemoryTransportBodyFormat(requestedMode)
			plan["transport_status"] = "provider_manager_marker_ready"
			plan["generation_status"] = "delegated_to_provider_manager"
			plan["token_estimate_status"] = "provider_manager_runtime_observation_pending"
		}
	} else if requestedMode != prepareTurnMemoryTransportModeText {
		if logicalMemory == "" {
			plan["generation_status"] = "empty"
			plan["error_code"] = "memory_pdf_empty"
		} else {
			var doc pdfmemory.Document
			var err error
			if generate == nil {
				err = errors.New("memory PDF generator unavailable")
			} else {
				doc, err = generate(logicalMemory)
			}
			if err != nil {
				plan["transport_status"] = "pdf_build_failed"
				plan["generation_status"] = "failed"
				plan["error_code"] = prepareTurnMemoryTransportPDFErrorCode(err)
			} else {
				encoded := base64.StdEncoding.EncodeToString(doc.Bytes)
				pdfHash := prepareTurnMemoryTransportPDFHash(doc.Bytes)
				estimatedPDFTokens := doc.PageCount * prepareTurnMemoryPDFTokensPerPage
				estimatedTokenDelta := estimatedTextTokens - estimatedPDFTokens
				estimatedTokenDeltaPercent := 0.0
				if estimatedTextTokens > 0 {
					estimatedTokenDeltaPercent = float64(estimatedTokenDelta) * 100 / float64(estimatedTextTokens)
				}
				filenameHash := strings.TrimPrefix(logicalHash, "sha256:")
				if len(filenameHash) > 16 {
					filenameHash = filenameHash[:16]
				}
				plan["selected_mode"] = requestedMode
				plan["body_format"] = prepareTurnMemoryTransportBodyFormat(requestedMode)
				plan["transport_status"] = "pdf_ready"
				plan["generation_status"] = "succeeded"
				plan["pdf_bytes_hash"] = pdfHash
				plan["filename"] = "archive-center-long-term-memory-" + filenameHash + ".pdf"
				plan["mime_type"] = prepareTurnMemoryPDFMIME
				plan["page_count"] = doc.PageCount
				plan["pdf_bytes"] = len(doc.Bytes)
				plan["base64_chars"] = len(encoded)
				plan["estimated_pdf_tokens"] = estimatedPDFTokens
				plan["estimated_token_delta"] = estimatedTokenDelta
				plan["estimated_token_delta_percent"] = estimatedTokenDeltaPercent
				plan["token_estimate_status"] = "documentation_estimate_unverified_provider_usage"
				plan["transient_payload_included"] = true
				plan["transient_payload_available"] = true
				transient = map[string]any{
					"contract_version": prepareTurnMemoryTransportPayloadContract,
					"pdf_base64":       encoded,
				}
			}
		}
	}

	planSeed := strings.Join([]string{
		requestCorrelationID,
		requestedMode,
		extractionStringFromAny(plan["selected_mode"]),
		logicalHash,
		extractionStringFromAny(plan["pdf_bytes_hash"]),
		extractionStringFromAny(plan["generation_status"]),
		extractionStringFromAny(plan["error_code"]),
		prepareTurnTextHash(auxiliaryText),
		prepareTurnTextHash(withoutLongTermMemory),
	}, "\n")
	planID := "mtp_" + strings.TrimPrefix(prepareTurnTextHash(planSeed), "sha256:")
	plan["plan_id"] = planID
	if transient != nil {
		transient["plan_id"] = planID
	}
	return plan, transient
}
