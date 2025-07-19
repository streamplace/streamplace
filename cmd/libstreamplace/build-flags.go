package main

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

var Version = os.Getenv("STREAMPLACE_DEV_VERSION")
var BuildTime = fmt.Sprint(time.Now().Unix())
var UUID = uuid.New().String()
