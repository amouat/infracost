package schema

import (
	"sort"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

var (
	HourToMonthUnitMultiplier = decimal.NewFromInt(730)
	MonthToHourUnitMultiplier = decimal.NewFromInt(1).Div(HourToMonthUnitMultiplier)
	DaysInMonth               = HourToMonthUnitMultiplier.DivRound(decimal.NewFromInt(24), 24)
	DayToMonthUnitMultiplier  = DaysInMonth.DivRound(HourToMonthUnitMultiplier, 24)
)

type ResourceFunc func(*ResourceData, *UsageData) *Resource

type Resource struct {
	Name              string
	CostComponents    []*CostComponent
	ActualCosts       []*ActualCosts
	SubResources      []*Resource
	HourlyCost        *decimal.Decimal
	MonthlyCost       *decimal.Decimal
	IsSkipped         bool
	NoPrice           bool
	SkipMessage       string
	ResourceType      string
	Tags              map[string]string
	UsageSchema       []*UsageItem
	EstimateUsage     EstimateFunc
	EstimationSummary map[string]bool
	Metadata          map[string]gjson.Result
}

func CalculateCosts(project *Project) {
	for _, r := range project.AllResources() {
		r.CalculateCosts()
	}
}

func (r *Resource) CalculateCosts() {
	h := decimal.Zero
	m := decimal.Zero
	hasCost := false

	for _, c := range r.CostComponents {
		c.CalculateCosts()
		if c.HourlyCost != nil || c.MonthlyCost != nil {
			hasCost = true
		}
		if c.HourlyCost != nil {
			h = h.Add(*c.HourlyCost)
		}
		if c.MonthlyCost != nil {
			m = m.Add(*c.MonthlyCost)
		}
	}

	for _, s := range r.SubResources {
		s.CalculateCosts()
		if s.HourlyCost != nil || s.MonthlyCost != nil {
			hasCost = true
		}
		if s.HourlyCost != nil {
			h = h.Add(*s.HourlyCost)
		}
		if s.MonthlyCost != nil {
			m = m.Add(*s.MonthlyCost)
		}
	}

	if hasCost {
		r.HourlyCost = &h
		r.MonthlyCost = &m
	}
	if r.NoPrice {
		log.Debugf("Skipping free resource %s", r.Name)
	}
}

func (r *Resource) FlattenedSubResources() []*Resource {
	resources := make([]*Resource, 0, len(r.SubResources))

	for _, s := range r.SubResources {
		resources = append(resources, s)

		if len(s.SubResources) > 0 {
			resources = append(resources, s.FlattenedSubResources()...)
		}
	}

	return resources
}

func (r *Resource) RemoveCostComponent(costComponent *CostComponent) {
	n := make([]*CostComponent, 0, len(r.CostComponents)-1)
	for _, c := range r.CostComponents {
		if c != costComponent {
			n = append(n, c)
		}
	}
	r.CostComponents = n
}

func SortResources(project *Project) {
	resources := project.AllResources()
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})
}

func MultiplyQuantities(resource *Resource, multiplier decimal.Decimal) {
	for _, costComponent := range resource.CostComponents {
		if costComponent.HourlyQuantity != nil {
			costComponent.HourlyQuantity = decimalPtr(costComponent.HourlyQuantity.Mul(multiplier))
		}
		if costComponent.MonthlyQuantity != nil {
			costComponent.MonthlyQuantity = decimalPtr(costComponent.MonthlyQuantity.Mul(multiplier))
		}
	}

	for _, subResource := range resource.SubResources {
		MultiplyQuantities(subResource, multiplier)
	}
}

func decimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}
