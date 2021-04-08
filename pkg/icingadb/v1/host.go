package v1

import (
	"bytes"
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
	"net"
)

type Host struct {
	Checkable   `json:",inline"`
	Address     string      `json:"address"`
	Address6    string      `json:"address6"`
	AddressBin  AddressBin  `json:"-"`
	Address6Bin Address6Bin `json:"-"`
}

// Init implements the contracts.Initer interface.
func (h *Host) Init() {
	h.Checkable.Init()
	h.AddressBin.Host = h
	h.Address6Bin.Host = h
}

type AddressBin struct {
	Host *Host `db:"-"`
}

var v4InV6Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

// Value implements the driver.Valuer interface.
func (ab AddressBin) Value() (driver.Value, error) {
	if ab.Host == nil {
		return nil, nil
	}

	ip := net.ParseIP(ab.Host.Address)
	if ip == nil {
		return nil, nil
	}

	if ip = bytes.TrimPrefix(ip, v4InV6Prefix); len(ip) == 4 {
		return []byte(ip), nil
	} else {
		return nil, nil
	}
}

type Address6Bin struct {
	Host *Host `db:"-"`
}

// Value implements the driver.Valuer interface.
func (ab Address6Bin) Value() (driver.Value, error) {
	if ab.Host == nil {
		return nil, nil
	}

	if ip := net.ParseIP(ab.Host.Address6); ip == nil {
		return nil, nil
	} else {
		return []byte(ip), nil
	}
}

type HostCustomvar struct {
	CustomvarMeta `json:",inline"`
	HostId        types.Binary `json:"object_id"`
}

type Hostgroup struct {
	GroupMeta `json:",inline"`
}

type HostgroupCustomvar struct {
	CustomvarMeta `json:",inline"`
	HostgroupId   types.Binary `json:"object_id"`
}

type HostgroupMember struct {
	MemberMeta  `json:",inline"`
	HostId      types.Binary `json:"object_id"`
	HostgroupId types.Binary `json:"group_id"`
}

func NewHost() contracts.Entity {
	return &Host{}
}

func NewHostCustomvar() contracts.Entity {
	return &HostCustomvar{}
}

func NewHostgroup() contracts.Entity {
	return &Hostgroup{}
}

func NewHostgroupCustomvar() contracts.Entity {
	return &HostgroupCustomvar{}
}

func NewHostgroupMember() contracts.Entity {
	return &HostgroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Host)(nil)
	_ driver.Valuer    = AddressBin{}
	_ driver.Valuer    = Address6Bin{}
	_ contracts.Initer = (*Hostgroup)(nil)
)
