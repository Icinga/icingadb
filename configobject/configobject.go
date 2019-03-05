package configobject

type Row interface {
	InsertValues() []interface{}
	UpdateValues() []interface{}
	GetId() string
	SetId(id string)
}

type RowFactory func() Row
