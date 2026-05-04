package config

type ConfigModel struct {
	GRPC      GRPCConfig     `yaml:"GRPC"`
	HTTP      HTTPConfig     `yaml:"HTTP"`
	Postgres  PostgresConfig `yaml:"Postgres"`
	JWT       JWTConfig      `yaml:"JWT"`
	OrderGRPC OrderGRPC      `yaml:"OrderGRPC"`
}

type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"DBName"`
	SSLMode  string `yaml:"sslMode"`
	PgDriver string `yaml:"pgDriver"`
}

type GRPCConfig struct {
	Host string `yaml:"host" validate:"required"`
	Port string `yaml:"port" validate:"required"`
}

type HTTPConfig struct {
	Host string `yaml:"host" validate:"required"`
	Port string `yaml:"port" validate:"required"`
}

type JWTConfig struct {
	Secret        string `yaml:"secret"`
	LeewaySeconds int64  `yaml:"leewaySeconds"`
}

type OrderGRPC struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}
