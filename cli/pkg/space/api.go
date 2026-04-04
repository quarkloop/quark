package space

// Init scaffolds a new space directory.
func Init(dir string) error { return runInit(dir) }

// Validate checks the Quarkfile and installed plugins for errors.
func Validate(dir string) error { return runValidate(dir) }
