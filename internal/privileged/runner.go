package privileged

type Runner interface {
	Run(command string) ([]byte, error)
}
