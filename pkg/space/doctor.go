package space

// DoctorIssue describes a single problem discovered by the doctor check.
type DoctorIssue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Message  string `json:"message"`
}

// DoctorResult is the structured outcome of a Quarkfile/plugin doctor run.
type DoctorResult struct {
	OK     bool          `json:"ok"`
	Issues []DoctorIssue `json:"issues"`
}
