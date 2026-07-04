package license

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Tier string

const (
	TierFree       Tier = "free"
	TierCloud      Tier = "cloud"
	TierEnterprise Tier = "enterprise"
)

type License struct {
	Tier      Tier
	Seats     int
	Org       string
	ExpiresAt time.Time
	Valid     bool
}

type claims struct {
	Tier  string `json:"tier"`
	Seats int    `json:"seats"`
	Org   string `json:"org"`
	jwt.RegisteredClaims
}

func Validate(key string) License {
	if key == "" {
		return freeTier()
	}

	secret := os.Getenv("ZFSDASH_LICENSE_SECRET")
	if secret == "" {
		return freeTier()
	}

	c := &claims{}
	token, err := jwt.ParseWithClaims(key, c, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return freeTier()
	}

	tier := TierFree
	seats := 1
	switch c.Tier {
	case "cloud":
		tier = TierCloud
		seats = 5
		if c.Seats > 0 {
			seats = c.Seats
		}
	case "enterprise":
		tier = TierEnterprise
		seats = 9999
		if c.Seats > 0 {
			seats = c.Seats
		}
	}

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	if c.ExpiresAt != nil {
		expiresAt = c.ExpiresAt.Time
	}

	return License{
		Tier:      tier,
		Seats:     seats,
		Org:       c.Org,
		ExpiresAt: expiresAt,
		Valid:     expiresAt.After(time.Now()),
	}
}

func freeTier() License {
	return License{Tier: TierFree, Seats: 1, Valid: false}
}

func Current() License {
	return Validate(os.Getenv("ZFSDASH_LICENSE_KEY"))
}

func IsEnterprise() bool {
	l := Current()
	return l.Tier == TierEnterprise && l.Valid
}

func IsCloud() bool {
	l := Current()
	return l.Tier == TierCloud && l.Valid
}

func MaxUsers() int {
	l := Current()
	if !l.Valid {
		return 1
	}
	return l.Seats
}
