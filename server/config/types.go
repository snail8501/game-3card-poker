package config

// Configuration object extracted from YAML configuration file.
type Configuration struct {
	Server Server              `mapstructure:"server"`
	User   User                `mapstructure:"user"`
	Smtp   SMTPConfiguration   `mapstructure:"smtp"`
	Redis  *RedisConfiguration `mapstructure:"redis"`
	Argon2 Argon2Password      `mapstructure:"argon2"`
}

type Server struct {
	Port int `mapstructure:"port"`
}

type User struct {
	DefaultHeadPic      []string `mapstructure:"defaultHeadPic"`
	ReceiveLimit        int64    `mapstructure:"receiveLimit"`
	ReceiveCount        int      `mapstructure:"receiveCount"`
	SignatureMessage    string   `mapstructure:"signatureMessage"`
	ReceiveAmount       int64    `mapstructure:"receiveAmount"`
	SignatureVerifyHost string   `mapstructure:"signatureVerifyHost"`
	DefaultBalance      int64    `mapstructure:"defaultBalance"`
}

// Argon2Password represents the argon2 hashing settings.
type Argon2Password struct {
	Variant     string `mapstructure:"variant"`
	Iterations  int    `mapstructure:"iterations"`
	Memory      int    `mapstructure:"memory"`
	Parallelism int    `mapstructure:"parallelism"`
	KeyLength   int    `mapstructure:"key_length"`
	SaltLength  int    `mapstructure:"salt_length"`
}

// RedisConfiguration represents the configuration related to redis session store.
type RedisConfiguration struct {
	Host              string `mapstructure:"host"`
	Port              int    `mapstructure:"port"`
	Password          string `mapstructure:"password"`
	DatabaseIndex     int    `mapstructure:"database_index"`
	MaximumActiveGame int    `mapstructure:"maximum_active_Game"`
	MinimumIdleGame   int    `mapstructure:"minimum_idle_Game"`
}

// SMTPConfiguration represents the configuration of the SMTP server to send emails with.
type SMTPConfiguration struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Identifier string `mapstructure:"identifier"`
	Sender     string `mapstructure:"sender"`
	Subject    string `mapstructure:"subject"`
}
