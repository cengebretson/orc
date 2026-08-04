package ticket

import "github.com/cengebretson/orc/internal/state"

type Ticket struct {
	FeatureDir string
	State      *state.State
}

func Load(root, query string) (*Ticket, error) {
	featureDir, err := state.FindFeatureDir(root, query)
	if err != nil {
		return nil, err
	}
	return load(featureDir)
}

func LoadWithArchive(root, query string) (*Ticket, error) {
	featureDir, err := state.FindFeatureDirWithArchive(root, query)
	if err != nil {
		return nil, err
	}
	return load(featureDir)
}

func Resolve(root, query string) (string, error) {
	return state.FindFeatureDir(root, query)
}

func load(featureDir string) (*Ticket, error) {
	s, err := state.Load(featureDir)
	if err != nil {
		return nil, err
	}
	return &Ticket{FeatureDir: featureDir, State: s}, nil
}
