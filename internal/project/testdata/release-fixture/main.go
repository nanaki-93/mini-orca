package fixture

func Run(name string) string {
	return "hello " + name
}

func Keep() string {
	return "unchanged"
}
