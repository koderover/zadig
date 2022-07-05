package step

type StepToolInstallSpec struct {
	Name      string   `bson:"name"                              json:"name"                                 yaml:"name"`
	Version   string   `bson:"version"                           json:"version"                              yaml:"version"`
	Scripts   []string `bson:"scripts"                           json:"scripts"                              yaml:"scripts"`
	BinPath   string   `bson:"bin_path"                          json:"bin_path"                             yaml:"bin_path"`
	Envs      []string `bson:"envs"                              json:"envs"                                 yaml:"envs"`
	Download  string   `bson:"download"                          json:"download"                             yaml:"download"`
	S3Storage *S3      `bson:"s3_storage"                        json:"s3_storage"                           yaml:"s3_storage"`
}
