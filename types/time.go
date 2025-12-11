package types

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"time"
)

type DbTime struct {
	time.Time
}

func (t DbTime) MarshalJSON() ([]byte, error) {
	seconds := t.Unix()
	return []byte(strconv.FormatInt(seconds, 10)), nil
}

func (t DbTime) Value() (driver.Value, error) {
	var zeroTime time.Time
	if t.Time.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *DbTime) Scan(v any) error {
	value, ok := v.(time.Time)
	if ok {
		*t = DbTime{Time: value}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}
