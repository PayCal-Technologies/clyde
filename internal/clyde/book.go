package clyde

import (
	"regexp"
	"strings"
	"time"
)

type BookPlan struct {
	Subject       string
	Timestamp     time.Time
	TitleOverride string
}

func NewBookPlan(subject string, now time.Time) (BookPlan, error) {
	cleaned := strings.Join(strings.Fields(subject), " ")
	if cleaned == "" {
		return BookPlan{}, errf("book subject must not be empty")
	}
	if now.IsZero() {
		now = time.Now()
	}
	return BookPlan{Subject: cleaned, Timestamp: now}, nil
}

func BookPlanFromTitle(title string) (BookPlan, error) {
	cleaned := strings.Join(strings.Fields(title), " ")
	if cleaned == "" {
		return BookPlan{}, errf("book title must not be empty")
	}
	return BookPlan{Subject: cleaned, Timestamp: time.Now(), TitleOverride: cleaned}, nil
}

func (p BookPlan) Title() string {
	if p.TitleOverride != "" {
		return p.TitleOverride
	}
	return p.Timestamp.Format("2006-01-02 1504") + " - " + p.Subject
}

func (p BookPlan) Slug() string {
	if p.TitleOverride != "" {
		re := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2}) (\d{4}) - (.+)$`)
		match := re.FindStringSubmatch(p.TitleOverride)
		if match != nil {
			stamp := match[1] + match[2] + match[3] + "-" + match[4]
			subject := slugText(match[5])
			if subject == "" {
				return stamp
			}
			return stamp + "-" + subject
		}
		return slugText(p.TitleOverride)
	}
	stamp := p.Timestamp.Format("20060102-1504")
	subject := slugText(p.Subject)
	if subject == "" {
		return stamp
	}
	return stamp + "-" + subject
}

func (p BookPlan) SourcePrefix() string {
	return p.Title() + " :: "
}

func slugText(value string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	return strings.Trim(re.ReplaceAllString(strings.ToLower(value), "-"), "-")
}
