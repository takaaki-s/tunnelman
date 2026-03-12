package tunnel

// Profile represents a named group of tunnels.
type Profile struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}
