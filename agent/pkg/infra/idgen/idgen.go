package idgen

import (
	"fmt"

	"github.com/google/uuid"
)

func New() string          { return uuid.New().String() }
func Short() string        { return uuid.New().String()[:8] }
func SpaceID() string      { return fmt.Sprintf("space-%s", Short()) }
