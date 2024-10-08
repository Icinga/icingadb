package v1

import (
	"database/sql/driver"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/contracts"
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

// Value implements the driver.Valuer interface.
func (ab AddressBin) Value() (driver.Value, error) {
	if ab.Host == nil {
		return nil, nil
	}

	if ip := net.ParseIP(ab.Host.Address).To4(); ip == nil {
		return nil, nil
	} else {
		return []byte(ip), nil
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
	HostId        types.Binary `json:"host_id"`
}

type HostState struct {
	State  `json:",inline"`
	HostId types.Binary `json:"host_id"`
}

type Hostgroup struct {
	GroupMeta `json:",inline"`
}

type HostgroupCustomvar struct {
	CustomvarMeta `json:",inline"`
	HostgroupId   types.Binary `json:"hostgroup_id"`
}

type HostgroupMember struct {
	MemberMeta  `json:",inline"`
	HostId      types.Binary `json:"host_id"`
	HostgroupId types.Binary `json:"hostgroup_id"`
}

func NewHost() database.Entity {
	return &Host{}
}

func NewHostCustomvar() database.Entity {
	return &HostCustomvar{}
}

func NewHostState() database.Entity {
	return &HostState{}
}

func NewHostgroup() database.Entity {
	return &Hostgroup{}
}

func NewHostgroupCustomvar() database.Entity {
	return &HostgroupCustomvar{}
}

func NewHostgroupMember() database.Entity {
	return &HostgroupMember{}
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Host)(nil)
	_ driver.Valuer    = AddressBin{}
	_ driver.Valuer    = Address6Bin{}
	_ contracts.Initer = (*Hostgroup)(nil)
)
