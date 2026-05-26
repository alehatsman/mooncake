package pilot

// Style selects which trailing TASK STYLE block buildSystemPrompt
// appends and which token set ParseResponse honors at the confirm
// gate (spec-67 §12.3, §10).
type Style string

const (
	// StylePlan is the historical default: one complete plan per turn,
	// executed inside a single transaction.
	StylePlan Style = "plan"
	// StyleStep is the iterative single-step style: one action per
	// turn, gated separately, result fed back to the next prompt.
	StyleStep Style = "step"
)

// promptStylePlan is the TASK STYLE block appended to the system
// prompt when Style == StylePlan. Verbatim from spec-67 §12.3.
const promptStylePlan = "TASK STYLE: complete plan\n" +
	"Design a complete mooncake YAML plan accomplishing this goal.\n" +
	"Output the entire plan in a single response; we execute the\n" +
	"whole plan in one transaction. Aim for 4–30 steps; verify with\n" +
	"`assert:` where useful."

// promptStyleStep is the TASK STYLE block appended when Style ==
// StyleStep. Plan §8 decision 3: no assert: hint in step mode — the
// model has no follow-up step in the same plan to validate against.
//
// The trailing "If LAST STEP STDOUT above answers the goal" sentence
// is the step-style-only counterpart to the captured-output prompt
// block (see prompt.go's Last Step Stdout rendering). Plan-style emits
// the whole plan in one turn, so per-iteration stdout isn't part of
// its mental model and the hint stays out of promptStylePlan.
const promptStyleStep = "TASK STYLE: one step at a time\n" +
	"Propose the NEXT SINGLE action needed to make progress toward\n" +
	"the goal. Output a YAML plan containing EXACTLY ONE step.\n" +
	"After we execute it and report back in LAST ITERATION, you\n" +
	"will propose the next single action. When the goal is reached,\n" +
	"emit an empty plan (the YAML literal `[]`) to signal \"done\".\n" +
	"If LAST STEP STDOUT above answers the goal, emit an empty plan\n" +
	"(`[]`) to signal done — do not re-propose the same diagnostic step."

// selectStyleFragment returns the TASK STYLE block for the given
// style. Unrecognized values fall back to StylePlan so a stray env
// var or pilot.yml typo still produces a coherent prompt.
func selectStyleFragment(style Style) string {
	switch style {
	case StyleStep:
		return promptStyleStep
	default:
		return promptStylePlan
	}
}
