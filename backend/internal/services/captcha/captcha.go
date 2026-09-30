package captcha

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "captcha:"

type Service struct {
	RDB *redis.Client
	TTL time.Duration
}

func New(rdb *redis.Client) *Service {
	return &Service{RDB: rdb, TTL: 5 * time.Minute}
}

type Challenge struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Hint     string `json:"hint"`
}

func (s *Service) Issue(ctx context.Context) (*Challenge, error) {
	a, _ := rand.Int(rand.Reader, big.NewInt(9))
	b, _ := rand.Int(rand.Reader, big.NewInt(9))
	x := int(a.Int64()) + 1
	y := int(b.Int64()) + 1
	ans := fmt.Sprintf("%d", x+y)
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	id := hex.EncodeToString(idBytes)
	if s.RDB != nil {
		if err := s.RDB.Set(ctx, keyPrefix+id, ans, s.TTL).Err(); err != nil {
			return nil, err
		}
	}
	return &Challenge{
		ID:       id,
		Question: fmt.Sprintf("%d + %d = ?", x, y),
		Hint:     "请填写计算结果",
	}, nil
}

func (s *Service) Verify(ctx context.Context, id, answer string) bool {
	if id == "" || answer == "" {
		return false
	}
	answer = strings.TrimSpace(answer)
	if s.RDB == nil {
		return true // dev fallback
	}
	key := keyPrefix + id
	got, err := s.RDB.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	_ = s.RDB.Del(ctx, key).Err() // one-time
	return got == answer
}
