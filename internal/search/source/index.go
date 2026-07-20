package source

type Match struct {
	FilePath string
	Language string
	Line     int
	Content  string
	Before   string
	After    string
	Score    float64
}
