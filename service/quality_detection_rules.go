package service

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"golang.org/x/net/html"
)

var qualityExternalCSSPattern = regexp.MustCompile(`(?i)(@import\s|url\s*\(\s*["']?\s*(https?:|//))`)

// qualityRuleEvidence records static observations, not browser execution or a
// subjective quality score. Even matching interactive elements need a genuine
// browser interaction test before any rules_passed result could be justified.
func qualityRuleEvidence(run *model.QualityProbeRun, document []byte) string {
	evidence := map[string]any{
		"rule_version": run.RuleVersion, "html_found": len(document) > 0,
		"generation_complete": run.Status == "completed", "within_output_limit": run.ErrorCode != "output_limit",
		"browser_interaction_verified": false, "review_reason": "visual_review_required",
	}
	if run.Rule == "interactive" {
		evidence["review_reason"] = "interaction_not_verified"
	}
	button, message, external, inStyle := false, false, false, false
	tokens := html.NewTokenizer(bytes.NewReader(document))
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			if tokens.Err() != io.EOF {
				evidence["parse_error"] = true
			}
			break
		}
		token := tokens.Token()
		if kind == html.TextToken && inStyle && qualityExternalCSSPattern.MatchString(token.Data) {
			external = true
		}
		if kind == html.EndTagToken && token.Data == "style" {
			inStyle = false
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		if token.Data == "style" {
			inStyle = true
		}
		for _, attr := range token.Attr {
			if attr.Key == "id" && attr.Val == "toggle-button" && token.Data == "button" {
				button = true
			}
			if attr.Key == "id" && attr.Val == "message" {
				message = true
			}
			if attr.Key == "style" && qualityExternalCSSPattern.MatchString(attr.Val) {
				external = true
			}
			switch attr.Key {
			case "src", "href", "poster", "action", "formaction", "data", "srcset":
				value := strings.ToLower(strings.TrimSpace(attr.Val))
				if value != "" && !strings.HasPrefix(value, "#") && !strings.HasPrefix(value, "data:") {
					external = true
				}
			}
		}
	}
	evidence["external_resource_references"] = external
	if run.RuleVersion == "button-v1" {
		evidence["required_elements_present"] = button && message
	}
	data, _ := common.Marshal(evidence)
	return string(data)
}
