package flow

type FieldValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type Field struct {
	Name  string     `json:"name"`
	Value FieldValue `json:"value"`
}

type EventValue struct {
	Id     string  `json:"id"`
	Fields []Field `json:"fields"`
}

type Event struct {
	Type  string     `json:"type"`
	Value EventValue `json:"value"`
}
