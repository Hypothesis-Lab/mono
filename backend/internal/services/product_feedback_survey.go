package services

import (
	"context"
	"fmt"

	"app/internal/clients"
	"app/internal/repositories/persona"
	"app/internal/repositories/productfeedbacksurvey"
)

const productFeedbackContext = `FocusLoop is a concept-stage family coordination app for urban professionals aged 28-45 with at least one school-age child. It combines a shared family calendar, weekly digest, lightweight task handoffs, and optional school email parsing.

Free includes two members and manual events. Plus costs $6.99 per month for up to six members, digest, and handoffs. Pro costs $11.99 per month for email parsing and a shared grocery list. Plus and Pro have a seven-day trial, annual billing saves about 35%, and Plus may have a $1 first-month promotion.`

type ProductFeedbackSurveyService struct {
	jev      *clients.JevClient
	personas *persona.Repository
	results  *productfeedbacksurvey.Repository
}

func NewProductFeedbackSurveyService(
	jev *clients.JevClient,
	personas *persona.Repository,
	results *productfeedbacksurvey.Repository,
) *ProductFeedbackSurveyService {
	return &ProductFeedbackSurveyService{
		jev:      jev,
		personas: personas,
		results:  results,
	}
}

func (s *ProductFeedbackSurveyService) Run(ctx context.Context) (int, error) {
	personas := s.personas.Query(persona.Query{})
	if len(personas) == 0 {
		return 0, fmt.Errorf("product feedback survey has no personas to process")
	}

	if err := s.results.DeleteAll(ctx); err != nil {
		return 0, fmt.Errorf("clear product feedback survey results: %w", err)
	}

	responses := make([]productfeedbacksurvey.Result, 0, len(personas))
	for _, selected := range personas {
		result, err := s.runPersona(ctx, selected)
		if err != nil {
			if saveErr := s.results.Save(ctx, responses); saveErr != nil {
				return len(responses), fmt.Errorf("save accumulated product feedback survey results after persona %s failed: %w", selected.ID, saveErr)
			}

			return len(responses), fmt.Errorf("run product feedback survey for persona %s: %w", selected.ID, err)
		}

		responses = append(responses, productfeedbacksurvey.Result{
			PersonaID:   selected.ID,
			PersonaName: selected.DisplayName,
			Response:    result.Raw,
		})
	}

	if err := s.results.Save(ctx, responses); err != nil {
		return len(responses), fmt.Errorf("save product feedback survey results: %w", err)
	}

	return len(responses), nil
}

func (s *ProductFeedbackSurveyService) List(ctx context.Context) ([]*productfeedbacksurvey.Result, error) {
	return s.results.List(ctx)
}

func (s *ProductFeedbackSurveyService) runPersona(ctx context.Context, selected *persona.Persona) (*clients.JevDecisionResponse, error) {
	result, err := s.jev.Decide(ctx, clients.JevDecisionRequest{
		State: map[string]any{
			"persona_profile": selected.Profile,
			"product_context": productFeedbackContext,
		},
		Questions: productFeedbackQuestions(),
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func productFeedbackQuestions() map[string]clients.JevDecisionQuestion {
	return map[string]clients.JevDecisionQuestion{
		"q0": choiceQuestion("After trying the free version, your realistic plan is...", map[string]string{
			"q0_use_free_wont_pay":      "Keep using free. I avoid paying when I can and would only upgrade if I absolutely have to.",
			"q0_pay_when_roi_clear":     "Start free, then upgrade once the time savings clearly justify the price.",
			"q0_subscribe_paid_launch":  "Subscribe to Plus or better at launch because I want the full experience immediately.",
			"q0_free_never_decide_tier": "Use free and probably never think much about tiers.",
			"q0_not_interested":         "Skip the rest because this product is not for me.",
		}),
		"q1": choiceQuestion("Plus vs Pro ($5 more per month)...", map[string]string{
			"q1_reject_both_tiers":        "I would not pay for either tier because paid plans are outside my budget.",
			"q1_plus_after_sustained_use": "Plus only after sustained use proves value; Pro is not worth the extra $5.",
			"q1_happy_plus_or_pro":        "I would happily take Plus or Pro; the extra $5 for Pro feels fine.",
			"q1_wont_compare_tiers":       "I would not spend effort comparing Plus vs Pro.",
		}),
		"q2": choiceQuestion("Annual vs monthly billing...", map[string]string{
			"q2_monthly_cancel_anytime": "Monthly only. I refuse to prepay and want to cancel the moment it stops being worth it.",
			"q2_annual_after_long_use":  "I would prepay annually only after a long trial proves steady value.",
			"q2_prepay_annual_plus":     "I would prepay annually on Plus upfront without waiting.",
			"q2_billing_no_preference":  "Monthly vs annual does not matter to me.",
		}),
		"q3": choiceQuestion("A limited $1 first-month Plus promo...", map[string]string{
			"q3_skip_even_one_dollar":  "I would skip it. It is not worth even $1 or the signup hassle.",
			"q3_one_dollar_try_cancel": "I would try it at $1 and cancel unless value becomes obvious within the month.",
			"q3_grab_dollar_promo":     "I would grab the $1 Plus month right away.",
			"q3_ignore_promo":          "I would ignore the promo unless I was already planning to upgrade.",
		}),
		"q4": choiceQuestion("A friend uses a paid organizer app...", map[string]string{
			"q4_seek_free_alternative":  "I would look for a free alternative first, even if the paid app seems good.",
			"q4_compare_pay_if_wins":    "I would compare a few options and pay only if one clearly beats staying free.",
			"q4_pay_best_no_hunt":       "I would pay for the best app without hunting for a free version first.",
			"q4_switch_only_effortless": "I would not switch unless it was basically handed to me ready to go.",
		}),
		"q5": choiceQuestion("Ads on free vs paying to remove them...", map[string]string{
			"q5_ads_not_worth_paying":   "Ads are annoying, but not enough to make me pay.",
			"q5_ads_pay_if_plus_useful": "Ads bother me, but I would pay only if Plus is useful beyond ad-free.",
			"q5_pay_primarily_adfree":   "Removing ads alone is enough value for me to pay.",
			"q5_ads_irrelevant_to_tier": "Ads would not factor into whether I upgrade.",
		}),
		"q6": choiceQuestion("Overall, the product's pricing feels...", map[string]string{
			"q6_too_expensive_stay_free": "Too expensive for what I would use; I would leave before paying.",
			"q6_fair_if_use_justifies":   "Fair only if my daily use clearly justifies the subscription.",
			"q6_premium_price_ok":        "Reasonable. I would pay for premium convenience.",
			"q6_pricing_unnoticed":       "I do not really notice or care about the pricing.",
		}),
		"overall_interest": {
			Type:         "score",
			Instructions: "Overall interest in this product. Rate the persona's interest from 1 (not interested) to 5 (very interested).",
			Criteria:     []string{"1", "2", "3", "4", "5"},
		},
		"would_try_beta": {
			Type:         "noul",
			Instructions: "Would the persona try a beta version?",
		},
	}
}

func choiceQuestion(prompt string, criteria map[string]string) clients.JevDecisionQuestion {
	return clients.JevDecisionQuestion{
		Type:         "choice",
		Instructions: prompt,
		Criteria:     criteria,
	}
}
