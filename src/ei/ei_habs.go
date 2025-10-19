package ei

// Hab structure for hab Data
type Hab struct {
	ID           int
	Name         string
	IconPath     string
	BaseCapacity float64
}

// Habs is a list of all habs in the game
var Habs = []Hab{
	{
		ID:           0,
		Name:         "Coop",
		IconPath:     "egginc/ei_hab_icon_coop.png",
		BaseCapacity: 250,
	},
	{
		ID:           1,
		Name:         "Shack",
		IconPath:     "egginc/ei_hab_icon_shack.png",
		BaseCapacity: 500,
	},
	{
		ID:           2,
		Name:         "Super Shack",
		IconPath:     "egginc/ei_hab_icon_super_shack.png",
		BaseCapacity: 1e3,
	},
	{
		ID:           3,
		Name:         "Short House",
		IconPath:     "egginc/ei_hab_icon_short_house.png",
		BaseCapacity: 2e3,
	},
	{
		ID:           4,
		Name:         "The Standard",
		IconPath:     "egginc/ei_hab_icon_the_standard.png",
		BaseCapacity: 5e3,
	},
	{
		ID:           5,
		Name:         "Long House",
		IconPath:     "egginc/ei_hab_icon_long_house.png",
		BaseCapacity: 1e4,
	},
	{
		ID:           6,
		Name:         "Double Decker",
		IconPath:     "egginc/ei_hab_icon_double_decker.png",
		BaseCapacity: 2e4,
	},
	{
		ID:           7,
		Name:         "Warehouse",
		IconPath:     "egginc/ei_hab_icon_warehouse.png",
		BaseCapacity: 5e4,
	},
	{
		ID:           8,
		Name:         "Center",
		IconPath:     "egginc/ei_hab_icon_center.png",
		BaseCapacity: 1e5,
	},
	{
		ID:           9,
		Name:         "Bunker",
		IconPath:     "egginc/ei_hab_icon_bunker.png",
		BaseCapacity: 2e5,
	},
	{
		ID:           10,
		Name:         "Eggkea",
		IconPath:     "egginc/ei_hab_icon_eggkea.png",
		BaseCapacity: 5e5,
	},
	{
		ID:           11,
		Name:         "HAB 1000",
		IconPath:     "egginc/ei_hab_icon_hab1k.png",
		BaseCapacity: 1e6,
	},
	{
		ID:           12,
		Name:         "Hangar",
		IconPath:     "egginc/ei_hab_icon_hanger.png",
		BaseCapacity: 2e6,
	},
	{
		ID:           13,
		Name:         "Tower",
		IconPath:     "egginc/ei_hab_icon_tower.png",
		BaseCapacity: 5e6,
	},
	{
		ID:           14,
		Name:         "HAB 10,000",
		IconPath:     "egginc/ei_hab_icon_hab10k.png",
		BaseCapacity: 1e7,
	},
	{
		ID:           15,
		Name:         "Eggtopia",
		IconPath:     "egginc/ei_hab_icon_eggtopia.png",
		BaseCapacity: 2.5e7,
	},
	{
		ID:           16,
		Name:         "Monolith",
		IconPath:     "egginc/ei_hab_icon_monolith.png",
		BaseCapacity: 5e7,
	},
	{
		ID:           17,
		Name:         "Planet Portal",
		IconPath:     "egginc/ei_hab_icon_portal.png",
		BaseCapacity: 1e8,
	},
	{
		ID:           18,
		Name:         "Chicken Universe",
		IconPath:     "egginc/ei_hab_icon_chicken_universe.png",
		BaseCapacity: 6e8,
	},
}

// IsPortalHab returns true if the hab is a portal hab
func IsPortalHab(hab Hab) bool {
	return hab.ID >= 17
}
