package step

type StepToolInstallSpec struct {
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version"`
	Scripts   []string `yaml:"scripts"`
	BinPath   string   `yaml:"bin_path"`
	Envs      []string `yaml:"envs"`
	Download  string   `yaml:"download"`
	S3Storage *S3      `yaml:"s3_storage"`
}
