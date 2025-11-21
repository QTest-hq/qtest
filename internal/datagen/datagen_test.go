package datagen

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewDataGenerator(t *testing.T) {
	gen := NewDataGenerator()

	if gen == nil {
		t.Fatal("NewDataGenerator() returned nil")
	}
	if gen.locale != "en" {
		t.Errorf("locale = %s, want en", gen.locale)
	}
}

func TestEmail(t *testing.T) {
	gen := NewDataGenerator()
	email := gen.Email()

	if !strings.Contains(email, "@") {
		t.Error("Email should contain @")
	}
	if !strings.Contains(email, ".com") {
		t.Error("Email should contain .com")
	}
}

func TestFirstName(t *testing.T) {
	gen := NewDataGenerator()
	name := gen.FirstName()

	if name == "" {
		t.Error("FirstName should not be empty")
	}
	found := false
	for _, fn := range firstNames {
		if fn == name {
			found = true
			break
		}
	}
	if !found {
		t.Error("FirstName should be from the firstNames list")
	}
}

func TestLastName(t *testing.T) {
	gen := NewDataGenerator()
	name := gen.LastName()

	if name == "" {
		t.Error("LastName should not be empty")
	}
}

func TestFullName(t *testing.T) {
	gen := NewDataGenerator()
	name := gen.FullName()

	if !strings.Contains(name, " ") {
		t.Error("FullName should contain a space")
	}
}

func TestUsername(t *testing.T) {
	gen := NewDataGenerator()
	username := gen.Username()

	if username == "" {
		t.Error("Username should not be empty")
	}
	if strings.ToLower(username) != username {
		t.Error("Username should be lowercase")
	}
}

func TestPassword(t *testing.T) {
	gen := NewDataGenerator()
	password := gen.Password()

	if len(password) != 12 {
		t.Errorf("Password length = %d, want 12", len(password))
	}
}

func TestPhone(t *testing.T) {
	gen := NewDataGenerator()
	phone := gen.Phone()

	if !strings.HasPrefix(phone, "+1-") {
		t.Error("Phone should start with +1-")
	}
}

func TestStreet(t *testing.T) {
	gen := NewDataGenerator()
	street := gen.Street()

	if street == "" {
		t.Error("Street should not be empty")
	}
	// Should contain number and suffix
	matched, _ := regexp.MatchString(`^\d+\s+\w+\s+(St|Ave|Blvd|Dr|Ln|Way)$`, street)
	if !matched {
		t.Logf("Street format: %s", street)
	}
}

func TestCity(t *testing.T) {
	gen := NewDataGenerator()
	city := gen.City()

	if city == "" {
		t.Error("City should not be empty")
	}
}

func TestState(t *testing.T) {
	gen := NewDataGenerator()
	state := gen.State()

	if state == "" {
		t.Error("State should not be empty")
	}
}

func TestCountry(t *testing.T) {
	gen := NewDataGenerator()
	country := gen.Country()

	if country == "" {
		t.Error("Country should not be empty")
	}
}

func TestZipCode(t *testing.T) {
	gen := NewDataGenerator()
	zip := gen.ZipCode()

	if len(zip) != 5 {
		t.Errorf("ZipCode length = %d, want 5", len(zip))
	}
}

func TestCompany(t *testing.T) {
	gen := NewDataGenerator()
	company := gen.Company()

	if company == "" {
		t.Error("Company should not be empty")
	}
}

func TestJobTitle(t *testing.T) {
	gen := NewDataGenerator()
	title := gen.JobTitle()

	if title == "" {
		t.Error("JobTitle should not be empty")
	}
}

func TestDepartment(t *testing.T) {
	gen := NewDataGenerator()
	dept := gen.Department()

	if dept == "" {
		t.Error("Department should not be empty")
	}
}

func TestUUID(t *testing.T) {
	gen := NewDataGenerator()
	uuid := gen.UUID()

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	matched, _ := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, uuid)
	if !matched {
		t.Errorf("UUID format invalid: %s", uuid)
	}
}

func TestSlug(t *testing.T) {
	gen := NewDataGenerator()
	slug := gen.Slug()

	if strings.Contains(slug, " ") {
		t.Error("Slug should not contain spaces")
	}
	if strings.ToLower(slug) != slug {
		t.Error("Slug should be lowercase")
	}
}

func TestDateTime(t *testing.T) {
	gen := NewDataGenerator()
	dt := gen.DateTime()

	// Should be RFC3339 format
	if !strings.Contains(dt, "T") {
		t.Error("DateTime should be RFC3339 format")
	}
}

func TestDate(t *testing.T) {
	gen := NewDataGenerator()
	d := gen.Date()

	// Should be YYYY-MM-DD format
	matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, d)
	if !matched {
		t.Errorf("Date format invalid: %s", d)
	}
}

func TestTime(t *testing.T) {
	gen := NewDataGenerator()
	tm := gen.Time()

	// Should be HH:MM:SS format
	matched, _ := regexp.MatchString(`^\d{2}:\d{2}:\d{2}$`, tm)
	if !matched {
		t.Errorf("Time format invalid: %s", tm)
	}
}

func TestURL(t *testing.T) {
	gen := NewDataGenerator()
	url := gen.URL()

	if !strings.HasPrefix(url, "https://") {
		t.Error("URL should start with https://")
	}
}

func TestImageURL(t *testing.T) {
	gen := NewDataGenerator()
	url := gen.ImageURL()

	if !strings.Contains(url, "picsum.photos") {
		t.Error("ImageURL should be from picsum.photos")
	}
}

func TestPrice(t *testing.T) {
	gen := NewDataGenerator()
	price := gen.Price()

	if price < 0 || price > 100 {
		t.Errorf("Price = %f, want 0-100", price)
	}
}

func TestCurrency(t *testing.T) {
	gen := NewDataGenerator()
	currency := gen.Currency()

	validCurrencies := map[string]bool{
		"USD": true, "EUR": true, "GBP": true, "JPY": true, "CAD": true, "AUD": true,
	}
	if !validCurrencies[currency] {
		t.Errorf("Currency = %s, not valid", currency)
	}
}

func TestWord(t *testing.T) {
	gen := NewDataGenerator()
	word := gen.Word()

	if word == "" {
		t.Error("Word should not be empty")
	}
}

func TestSentence(t *testing.T) {
	gen := NewDataGenerator()
	sentence := gen.Sentence()

	if !strings.HasSuffix(sentence, ".") {
		t.Error("Sentence should end with period")
	}
	// First letter should be uppercase
	if sentence[0] < 'A' || sentence[0] > 'Z' {
		t.Error("Sentence should start with uppercase")
	}
}

func TestParagraph(t *testing.T) {
	gen := NewDataGenerator()
	para := gen.Paragraph()

	// Should contain multiple sentences
	if strings.Count(para, ".") < 2 {
		t.Error("Paragraph should contain multiple sentences")
	}
}

func TestTitle(t *testing.T) {
	gen := NewDataGenerator()
	title := gen.Title()

	// Should have multiple words
	if !strings.Contains(title, " ") {
		t.Error("Title should contain multiple words")
	}
}

func TestInt(t *testing.T) {
	gen := NewDataGenerator()

	for i := 0; i < 100; i++ {
		val := gen.Int(10, 20)
		if val < 10 || val > 20 {
			t.Errorf("Int(%d, %d) = %d, out of range", 10, 20, val)
		}
	}
}

func TestFloat(t *testing.T) {
	gen := NewDataGenerator()

	for i := 0; i < 100; i++ {
		val := gen.Float(1.0, 5.0)
		if val < 1.0 || val > 5.0 {
			t.Errorf("Float(1, 5) = %f, out of range", val)
		}
	}
}

func TestBool(t *testing.T) {
	gen := NewDataGenerator()

	trueCount := 0
	falseCount := 0
	for i := 0; i < 100; i++ {
		if gen.Bool() {
			trueCount++
		} else {
			falseCount++
		}
	}
	// Should have a mix of both
	if trueCount == 0 || falseCount == 0 {
		t.Error("Bool should generate both true and false")
	}
}

func TestAge(t *testing.T) {
	gen := NewDataGenerator()

	for i := 0; i < 100; i++ {
		age := gen.Age()
		if age < 18 || age > 77 {
			t.Errorf("Age = %d, out of range 18-77", age)
		}
	}
}

func TestStatus(t *testing.T) {
	gen := NewDataGenerator()
	status := gen.Status()

	validStatuses := map[string]bool{
		"active": true, "inactive": true, "pending": true, "completed": true, "cancelled": true,
	}
	if !validStatuses[status] {
		t.Errorf("Status = %s, not valid", status)
	}
}

func TestGenerateForType_Email(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "userEmail")

	email, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if !strings.Contains(email, "@") {
		t.Error("Should generate email for email field")
	}
}

func TestGenerateForType_FirstName(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "firstName")

	name, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if name == "" {
		t.Error("Should generate first name")
	}
}

func TestGenerateForType_LastName(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "lastName")

	name, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if name == "" {
		t.Error("Should generate last name")
	}
}

func TestGenerateForType_Username(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "username")

	username, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if username == "" {
		t.Error("Should generate username")
	}
}

func TestGenerateForType_ID(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "userId")

	id, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if !strings.Contains(id, "-") {
		t.Error("ID should be UUID format")
	}
}

func TestGenerateForType_Date(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "createdDate")

	date, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if !strings.Contains(date, "T") {
		t.Error("Date should be RFC3339 format")
	}
}

func TestGenerateForType_Price(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("float", "price")

	price, ok := result.(float64)
	if !ok {
		t.Fatal("Should return float64")
	}
	if price < 0 {
		t.Error("Price should be positive")
	}
}

func TestGenerateForType_Status(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "status")

	status, ok := result.(string)
	if !ok {
		t.Fatal("Should return string")
	}
	if status == "" {
		t.Error("Should generate status")
	}
}

func TestGenerateForType_TypeFallback_String(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("string", "unknownField")

	_, ok := result.(string)
	if !ok {
		t.Error("String type should return string")
	}
}

func TestGenerateForType_TypeFallback_Int(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("int", "unknownField")

	_, ok := result.(int)
	if !ok {
		t.Error("Int type should return int")
	}
}

func TestGenerateForType_TypeFallback_Float(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("float", "unknownField")

	_, ok := result.(float64)
	if !ok {
		t.Error("Float type should return float64")
	}
}

func TestGenerateForType_TypeFallback_Bool(t *testing.T) {
	gen := NewDataGenerator()
	result := gen.GenerateForType("bool", "unknownField")

	_, ok := result.(bool)
	if !ok {
		t.Error("Bool type should return bool")
	}
}

func TestPick(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := pick(items)

	found := false
	for _, item := range items {
		if item == result {
			found = true
			break
		}
	}
	if !found {
		t.Error("Pick should return item from list")
	}
}

func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("Should contain 'world'")
	}
	if contains("hello", "world") {
		t.Error("Should not contain 'world'")
	}
}
