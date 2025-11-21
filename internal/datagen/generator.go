package datagen

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// DataGenerator generates realistic test data
type DataGenerator struct {
	locale string
}

// NewDataGenerator creates a new data generator
func NewDataGenerator() *DataGenerator {
	return &DataGenerator{
		locale: "en",
	}
}

// GenerateForType generates data based on type/field name
func (g *DataGenerator) GenerateForType(typeName string, fieldName string) interface{} {
	// Normalize for matching
	typeNameLower := strings.ToLower(typeName)
	fieldNameLower := strings.ToLower(fieldName)

	// Check field name first for semantic matching
	switch {
	// Personal info
	case contains(fieldNameLower, "email"):
		return g.Email()
	case contains(fieldNameLower, "name") && contains(fieldNameLower, "first"):
		return g.FirstName()
	case contains(fieldNameLower, "name") && contains(fieldNameLower, "last"):
		return g.LastName()
	case contains(fieldNameLower, "name") && !contains(fieldNameLower, "user"):
		return g.FullName()
	case contains(fieldNameLower, "username"):
		return g.Username()
	case contains(fieldNameLower, "password"):
		return g.Password()
	case contains(fieldNameLower, "phone"):
		return g.Phone()

	// Address
	case contains(fieldNameLower, "street") || contains(fieldNameLower, "address"):
		return g.Street()
	case contains(fieldNameLower, "city"):
		return g.City()
	case contains(fieldNameLower, "state"):
		return g.State()
	case contains(fieldNameLower, "country"):
		return g.Country()
	case contains(fieldNameLower, "zip") || contains(fieldNameLower, "postal"):
		return g.ZipCode()

	// Business
	case contains(fieldNameLower, "company"):
		return g.Company()
	case contains(fieldNameLower, "title") && contains(fieldNameLower, "job"):
		return g.JobTitle()
	case contains(fieldNameLower, "department"):
		return g.Department()

	// IDs
	case contains(fieldNameLower, "id") || contains(fieldNameLower, "uuid"):
		return g.UUID()
	case contains(fieldNameLower, "slug"):
		return g.Slug()

	// Dates
	case contains(fieldNameLower, "date") || contains(fieldNameLower, "created") || contains(fieldNameLower, "updated"):
		return g.DateTime()
	case contains(fieldNameLower, "time"):
		return g.Time()

	// URLs/Links
	case contains(fieldNameLower, "url") || contains(fieldNameLower, "link"):
		return g.URL()
	case contains(fieldNameLower, "image") || contains(fieldNameLower, "avatar"):
		return g.ImageURL()

	// Money
	case contains(fieldNameLower, "price") || contains(fieldNameLower, "amount") || contains(fieldNameLower, "cost"):
		return g.Price()
	case contains(fieldNameLower, "currency"):
		return g.Currency()

	// Text
	case contains(fieldNameLower, "description") || contains(fieldNameLower, "bio"):
		return g.Paragraph()
	case contains(fieldNameLower, "title"):
		return g.Title()
	case contains(fieldNameLower, "comment") || contains(fieldNameLower, "message"):
		return g.Sentence()

	// Numbers
	case contains(fieldNameLower, "age"):
		return g.Age()
	case contains(fieldNameLower, "quantity") || contains(fieldNameLower, "count"):
		return g.Int(1, 100)
	case contains(fieldNameLower, "rating") || contains(fieldNameLower, "score"):
		return g.Float(1, 5)
	case contains(fieldNameLower, "percent"):
		return g.Float(0, 100)

	// Status
	case contains(fieldNameLower, "status"):
		return g.Status()
	case contains(fieldNameLower, "active") || contains(fieldNameLower, "enabled"):
		return g.Bool()
	}

	// Fall back to type-based generation
	switch typeNameLower {
	case "string":
		return g.Word()
	case "int", "int32", "int64", "integer", "number":
		return g.Int(1, 1000)
	case "float", "float32", "float64", "double", "decimal":
		return g.Float(0, 1000)
	case "bool", "boolean":
		return g.Bool()
	case "date", "datetime", "timestamp":
		return g.DateTime()
	case "email":
		return g.Email()
	case "url", "uri":
		return g.URL()
	case "uuid":
		return g.UUID()
	default:
		return g.Word()
	}
}

// Personal Info Generators

func (g *DataGenerator) Email() string {
	return fmt.Sprintf("%s.%s@%s.com",
		strings.ToLower(g.FirstName()),
		strings.ToLower(g.LastName()),
		pick([]string{"gmail", "yahoo", "outlook", "example", "test"}))
}

func (g *DataGenerator) FirstName() string {
	return pick(firstNames)
}

func (g *DataGenerator) LastName() string {
	return pick(lastNames)
}

func (g *DataGenerator) FullName() string {
	return g.FirstName() + " " + g.LastName()
}

func (g *DataGenerator) Username() string {
	return strings.ToLower(g.FirstName()) + fmt.Sprintf("%d", rand.Intn(999))
}

func (g *DataGenerator) Password() string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	b := make([]byte, 12)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func (g *DataGenerator) Phone() string {
	return fmt.Sprintf("+1-%03d-%03d-%04d", rand.Intn(999), rand.Intn(999), rand.Intn(9999))
}

// Address Generators

func (g *DataGenerator) Street() string {
	return fmt.Sprintf("%d %s %s",
		rand.Intn(9999)+1,
		pick(streetNames),
		pick([]string{"St", "Ave", "Blvd", "Dr", "Ln", "Way"}))
}

func (g *DataGenerator) City() string {
	return pick(cities)
}

func (g *DataGenerator) State() string {
	return pick(states)
}

func (g *DataGenerator) Country() string {
	return pick(countries)
}

func (g *DataGenerator) ZipCode() string {
	return fmt.Sprintf("%05d", rand.Intn(99999))
}

// Business Generators

func (g *DataGenerator) Company() string {
	return pick(companies)
}

func (g *DataGenerator) JobTitle() string {
	return pick(jobTitles)
}

func (g *DataGenerator) Department() string {
	return pick(departments)
}

// ID Generators

func (g *DataGenerator) UUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (g *DataGenerator) Slug() string {
	return strings.ToLower(strings.ReplaceAll(g.Title(), " ", "-"))
}

// Date/Time Generators

func (g *DataGenerator) DateTime() string {
	t := time.Now().AddDate(0, 0, -rand.Intn(365))
	return t.Format(time.RFC3339)
}

func (g *DataGenerator) Date() string {
	t := time.Now().AddDate(0, 0, -rand.Intn(365))
	return t.Format("2006-01-02")
}

func (g *DataGenerator) Time() string {
	return fmt.Sprintf("%02d:%02d:%02d", rand.Intn(24), rand.Intn(60), rand.Intn(60))
}

// URL Generators

func (g *DataGenerator) URL() string {
	return fmt.Sprintf("https://%s.com/%s",
		strings.ToLower(pick(companies)),
		g.Slug())
}

func (g *DataGenerator) ImageURL() string {
	return fmt.Sprintf("https://picsum.photos/seed/%d/200/200", rand.Intn(1000))
}

// Money Generators

func (g *DataGenerator) Price() float64 {
	return float64(rand.Intn(10000)) / 100
}

func (g *DataGenerator) Currency() string {
	return pick([]string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD"})
}

// Text Generators

func (g *DataGenerator) Word() string {
	return pick(words)
}

func (g *DataGenerator) Sentence() string {
	wordCount := rand.Intn(10) + 5
	var w []string
	for i := 0; i < wordCount; i++ {
		w = append(w, g.Word())
	}
	s := strings.Join(w, " ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

func (g *DataGenerator) Paragraph() string {
	sentenceCount := rand.Intn(3) + 2
	var sentences []string
	for i := 0; i < sentenceCount; i++ {
		sentences = append(sentences, g.Sentence())
	}
	return strings.Join(sentences, " ")
}

func (g *DataGenerator) Title() string {
	wordCount := rand.Intn(3) + 2
	var w []string
	for i := 0; i < wordCount; i++ {
		word := g.Word()
		w = append(w, strings.ToUpper(word[:1])+word[1:])
	}
	return strings.Join(w, " ")
}

// Number Generators

func (g *DataGenerator) Int(min, max int) int {
	return rand.Intn(max-min+1) + min
}

func (g *DataGenerator) Float(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func (g *DataGenerator) Bool() bool {
	return rand.Intn(2) == 1
}

func (g *DataGenerator) Age() int {
	return rand.Intn(60) + 18
}

func (g *DataGenerator) Status() string {
	return pick([]string{"active", "inactive", "pending", "completed", "cancelled"})
}

// Helper functions

func pick(items []string) string {
	return items[rand.Intn(len(items))]
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Data sets

var firstNames = []string{
	"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
	"William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Charles", "Karen", "Emma", "Olivia", "Ava", "Sophia", "Liam", "Noah",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
}

var streetNames = []string{
	"Main", "Oak", "Pine", "Maple", "Cedar", "Elm", "Washington", "Lake",
	"Hill", "Park", "River", "Spring", "Valley", "Forest", "Sunset", "Highland",
}

var cities = []string{
	"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia",
	"San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville",
	"Seattle", "Denver", "Boston", "Portland", "Miami", "Atlanta", "Minneapolis",
}

var states = []string{
	"California", "Texas", "Florida", "New York", "Pennsylvania", "Illinois",
	"Ohio", "Georgia", "North Carolina", "Michigan", "New Jersey", "Virginia",
}

var countries = []string{
	"United States", "Canada", "United Kingdom", "Germany", "France", "Australia",
	"Japan", "Brazil", "India", "Mexico", "Spain", "Italy", "Netherlands", "Sweden",
}

var companies = []string{
	"Acme", "Globex", "Initech", "Umbrella", "Stark", "Wayne", "Oscorp", "Cyberdyne",
	"Aperture", "Weyland", "Tyrell", "Massive", "Dynamic", "Infinite", "Quantum",
}

var jobTitles = []string{
	"Software Engineer", "Product Manager", "Designer", "Data Analyst", "DevOps Engineer",
	"QA Engineer", "Tech Lead", "Engineering Manager", "CTO", "VP of Engineering",
	"Frontend Developer", "Backend Developer", "Full Stack Developer", "Architect",
}

var departments = []string{
	"Engineering", "Product", "Design", "Marketing", "Sales", "Operations",
	"Human Resources", "Finance", "Legal", "Customer Success", "Support",
}

var words = []string{
	"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit",
	"sed", "do", "eiusmod", "tempor", "incididunt", "labore", "dolore", "magna",
	"aliqua", "enim", "minim", "veniam", "quis", "nostrud", "exercitation", "ullamco",
	"test", "data", "sample", "example", "demo", "mock", "fake", "random", "generated",
}
