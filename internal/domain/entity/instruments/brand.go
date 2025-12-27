package instruments

import "github.com/google/uuid"

type Brand struct {
	UID         uuid.UUID
	Name        string
	Description string
	Info        string
	CompanyUID  uuid.UUID
	SectorUID   uuid.UUID
	CountryCode string
}

type Company struct {
	UID  uuid.UUID
	Name string
}

type Sector struct {
	UID        uuid.UUID
	Name       string
	Volatility int32
}

type Country struct {
	AlfaTwo   string
	AlfaThree string
	Name      string
	NameBrief string
}
