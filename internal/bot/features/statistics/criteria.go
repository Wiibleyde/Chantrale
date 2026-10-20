package statistics

import "time"

// Criterion describes what a leaderboard ranks members by.
type Criterion struct {
	Key   string
	Label string
	Title string
	// Duration criteria sum duty sessions of RoleType; others count EventType rows.
	Duration  bool
	RoleType  string
	EventType string
	Singular  string
	Plural    string
}

var criteria = []Criterion{
	{Key: "service", Label: "Temps de service", Title: "⏱️ Temps de service", Duration: true, RoleType: "duty"},
	{Key: "semi_service", Label: "Temps de semi service", Title: "🟡 Temps de semi service", Duration: true, RoleType: "oncall"},
	{Key: "labo", Label: "Analyses de laboratoire", Title: "🧪 Analyses de laboratoire", EventType: "labo.test_start", Singular: "analyse", Plural: "analyses"},
	{Key: "lits", Label: "Admissions en lit", Title: "🛏️ Admissions en lit", EventType: "bed.assign", Singular: "admission", Plural: "admissions"},
	{Key: "morgue", Label: "Corps pris en charge (morgue)", Title: "🪦 Corps pris en charge", EventType: "mortuary.assign", Singular: "corps", Plural: "corps"},
	{Key: "formations", Label: "Dossiers de formation", Title: "🎓 Dossiers de formation", EventType: "doctor.dossier_create", Singular: "dossier", Plural: "dossiers"},
}

func findCriterion(key string) (Criterion, bool) {
	for _, c := range criteria {
		if c.Key == key {
			return c, true
		}
	}
	return Criterion{}, false
}

// Period is a time window ending now; a zero Window means all time.
type Period struct {
	Key         string
	Label       string
	Description string
	Window      time.Duration
}

const defaultPeriodKey = "7j"

var periods = []Period{
	{Key: "24h", Label: "Dernières 24 heures", Description: "les dernières 24 heures", Window: 24 * time.Hour},
	{Key: "7j", Label: "7 derniers jours", Description: "les 7 derniers jours", Window: 7 * 24 * time.Hour},
	{Key: "30j", Label: "30 derniers jours", Description: "les 30 derniers jours", Window: 30 * 24 * time.Hour},
	{Key: "tout", Label: "Depuis le début", Description: "depuis le début"},
}

func findPeriod(key string) (Period, bool) {
	for _, p := range periods {
		if p.Key == key {
			return p, true
		}
	}
	return Period{}, false
}

// Since returns the start of the period, or the zero time for all time.
func (p Period) Since(now time.Time) time.Time {
	if p.Window == 0 {
		return time.Time{}
	}
	return now.Add(-p.Window)
}
