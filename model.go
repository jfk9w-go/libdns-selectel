package selectel

import (
	"time"

	"github.com/libdns/libdns"
)

type Record struct {
	ID   string
	Name string
	TTL  time.Duration
	Type string
	Data string
}

func (r Record) RR() libdns.RR {
	return libdns.RR{
		Name: r.Name,
		TTL:  r.TTL,
		Type: r.Type,
		Data: r.Data,
	}
}
