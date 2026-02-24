package agent

type Snapshot struct {
	Branch       string   `json:"branch"`
	Head         string   `json:"head"`
	Clean        bool     `json:"clean"`
	TopLevelDirs []string `json:"top_level_dirs"`
	Actions      []string `json:"actions"`
}

type IterationLog struct {
	Iteration        int      `json:"iteration"`
	Goal             string   `json:"goal"`
	PlanHash         string   `json:"plan_hash"`
	Status           string   `json:"status"`
	ChangedFiles     []string `json:"changed_files"`
	DiffStat         DiffStat `json:"diff_stat"`
	Artifacts        []string `json:"artifacts"`
	Provider         string   `json:"provider,omitempty"`
	Model            string   `json:"model,omitempty"`
	ValidationError  string   `json:"validation_error,omitempty"`
	ExecutionError   string   `json:"execution_error,omitempty"`
	AssertionsFailed int      `json:"assertions_failed,omitempty"`
}

type DiffStat struct {
	Files      int `json:"files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

type RunOptions struct {
	Goal          string
	PlanPath      string
	UseStdin      bool
	RepoRoot      string
	Provider      string
	Model         string
	MaxIterations int
}

type PlanInput struct {
	Goal          string
	Snapshot      []byte
	LastIteration *IterationSummary
}

type IterationSummary struct {
	Iteration    int
	PlanHash     string
	Status       string
	ChangedFiles []string
	ErrorMessage string
}

type StopReason string

const (
	StopSuccess    StopReason = "success"
	StopNoProgress StopReason = "no_progress"
	StopNoChange   StopReason = "no_change"
	StopFailed     StopReason = "failed"
	StopMaxReached StopReason = "max_iterations"
)
