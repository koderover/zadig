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

type S3 struct {
	Ak        string `yaml:"ak"`
	Sk        string `yaml:"sk"`
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Subfolder string `yaml:"subfolder"`
	Insecure  bool   `yaml:"insecure"`
	Provider  int    `yaml:"provider"`
	Protocol  string `yaml:"protocol"`
}
