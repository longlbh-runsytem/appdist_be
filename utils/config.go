package utils

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DBSource string `mapstructure:"DB_SOURCE"`

	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`

	// PublicBaseURL là URL HTTPS mà iPhone của khách truy cập được. iOS chỉ
	// chấp nhận itms-services và .mobileconfig qua HTTPS có chứng chỉ hợp lệ,
	// nên KHÔNG dùng được địa chỉ nội bộ hay chứng chỉ tự ký.
	PublicBaseURL string `mapstructure:"PUBLIC_BASE_URL"`

	// Domain (vd https://dist.congty.vn hoặc URL ngrok) — nếu có thì ghi đè
	// PublicBaseURL. Các URL thiết bị gọi thẳng vào BE (nút Cài đặt,
	// manifest.plist, file .ipa/.apk, icon) đều dựng từ giá trị này.
	Domain string `mapstructure:"DOMAIN"`

	TokenSymmetricKey    string        `mapstructure:"TOKEN_SYMMETRIC_KEY"`
	AccessTokenDuration  time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`
	RefreshTokenDuration time.Duration `mapstructure:"REFRESH_TOKEN_DURATION"`

	// Backblaze B2 qua S3-compatible API. Endpoint KHÔNG có "https://",
	// vd s3.eu-central-003.backblazeb2.com; Region là phần giữa, vd eu-central-003.
	B2Endpoint       string `mapstructure:"B2_ENDPOINT"`
	B2Region         string `mapstructure:"B2_REGION"`
	B2Bucket         string `mapstructure:"B2_BUCKET"`
	B2KeyID          string `mapstructure:"B2_KEY_ID"`
	B2ApplicationKey string `mapstructure:"B2_APPLICATION_KEY"`

	// MaxUploadSizeMB giới hạn dung lượng 1 file upload. 0 = mặc định 1024.
	MaxUploadSizeMB int64 `mapstructure:"MAX_UPLOAD_SIZE_MB"`

	// FrontendURL là gốc của web FE, dùng dựng link đưa cho người dùng: trang
	// tải (<FrontendURL>/d/<token>, cả trong QR) và link mời
	// (<FrontendURL>/accept-invite?token=...). Bỏ trống = PublicBaseURL.
	FrontendURL string `mapstructure:"FRONTEND_URL"`

	// CORSAllowedOrigins: các origin web FE được gọi API, phân tách bằng dấu
	// phẩy, vd "https://app.example.com,http://localhost:5173". "*" = mọi
	// origin (chỉ nên dùng khi dev). Bỏ trống = chỉ FrontendURL.
	CORSAllowedOriginsRaw string   `mapstructure:"CORS_ALLOWED_ORIGINS"`
	CORSAllowedOrigins    []string `mapstructure:"-"`

	// SwaggerEnabled bật Swagger UI tại /swagger/index.html.
	SwaggerEnabled bool `mapstructure:"SWAGGER_ENABLED"`
}

// MaxUploadSize trả giới hạn upload theo byte.
func (c Config) MaxUploadSize() int64 {
	if c.MaxUploadSizeMB <= 0 {
		return 1024 << 20
	}
	return c.MaxUploadSizeMB << 20
}

// LoadConfig đọc cấu hình từ file <name>.env trong path, rồi cho biến môi
// trường ghi đè lên.
//
// Thiếu file KHÔNG phải lỗi (môi trường production thường chỉ có biến môi
// trường), nhưng mọi lỗi khác thì trả về.
func LoadConfig(path, name string) (Config, error) {
	var config Config

	viper.AddConfigPath(path)
	viper.SetConfigName(name)
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return config, err
		}
	}

	// AutomaticEnv không đủ để Unmarshal thấy biến môi trường khi không có
	// file config: phải bind tường minh từng key.
	for _, key := range configKeys() {
		if err := viper.BindEnv(key); err != nil {
			return config, err
		}
	}

	if err := viper.Unmarshal(&config); err != nil {
		return config, err
	}

	config.DBSource = strings.TrimSpace(config.DBSource)
	if domain := strings.TrimSpace(config.Domain); domain != "" {
		config.PublicBaseURL = domain
	}
	config.PublicBaseURL = strings.TrimRight(strings.TrimSpace(config.PublicBaseURL), "/")
	config.FrontendURL = strings.TrimRight(strings.TrimSpace(config.FrontendURL), "/")
	if config.FrontendURL == "" {
		config.FrontendURL = config.PublicBaseURL
	}

	for _, o := range strings.Split(config.CORSAllowedOriginsRaw, ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			config.CORSAllowedOrigins = append(config.CORSAllowedOrigins, o)
		}
	}
	if len(config.CORSAllowedOrigins) == 0 {
		config.CORSAllowedOrigins = []string{config.FrontendURL}
	}
	return config, nil
}

func configKeys() []string {
	return []string{
		"DB_SOURCE",
		"HTTP_SERVER_ADDRESS", "PUBLIC_BASE_URL", "DOMAIN",
		"TOKEN_SYMMETRIC_KEY", "ACCESS_TOKEN_DURATION", "REFRESH_TOKEN_DURATION",
		"B2_ENDPOINT", "B2_REGION", "B2_BUCKET", "B2_KEY_ID", "B2_APPLICATION_KEY",
		"MAX_UPLOAD_SIZE_MB",
		"SWAGGER_ENABLED", "FRONTEND_URL", "CORS_ALLOWED_ORIGINS",
	}
}
