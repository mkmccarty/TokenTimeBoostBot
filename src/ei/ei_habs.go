package ei

// Hab structure for hab Data
type Hab struct {
	ID           int
	Name         string
	IconPath     string
	BaseCapacity float64
	Prices       [4]float64
	Gems         [4]float64
}

// Habs is a list of all habs in the game
var Habs = []Hab{
	{
		ID:           0,
		Name:         "Coop",
		IconPath:     "egginc/ei_hab_icon_coop.png",
		BaseCapacity: 250,
		Prices:       [4]float64{0, 29.12, 56.37, 96.56},
		Gems:         [4]float64{0, 124, 195, 269},
	},
	{
		ID:           1,
		Name:         "Shack",
		IconPath:     "egginc/ei_hab_icon_shack.png",
		BaseCapacity: 500,
		Prices:       [4]float64{467, 803, 1256, 1861},
		Gems:         [4]float64{917, 1211, 1528, 2235},
	},
	{
		ID:           2,
		Name:         "Super Shack",
		IconPath:     "egginc/ei_hab_icon_super_shack.png",
		BaseCapacity: 1e3,
		Prices:       [4]float64{12267, 23531, 40899, 66811},
		Gems:         [4]float64{7664, 10501, 13739, 17451},
	},
	{
		ID:           3,
		Name:         "Short House",
		IconPath:     "egginc/ei_hab_icon_short_house.png",
		BaseCapacity: 2e3,
		Prices:       [4]float64{340829, 733773, 1418851, 2551205},
		Gems:         [4]float64{46272, 67413, 93773, 126139},
	},
	{
		ID:           4,
		Name:         "The Standard",
		IconPath:     "egginc/ei_hab_icon_the_standard.png",
		BaseCapacity: 5e3,
		Prices:       [4]float64{21440013, 68130181, 178.166e6, 408.981e6},
		Gems:         [4]float64{420445, 793421, 1370219, 2225192},
	},
	{
		ID:           5,
		Name:         "Long House",
		IconPath:     "egginc/ei_hab_icon_long_house.png",
		BaseCapacity: 1e4,
		Prices:       [4]float64{2.321e9, 5.957e9, 13.347e9, 27.195e9},
		Gems:         [4]float64{6337328, 11432715, 19255856, 30763853},
	},
	{
		ID:           6,
		Name:         "Double Decker",
		IconPath:     "egginc/ei_hab_icon_double_decker.png",
		BaseCapacity: 2e4,
		Prices:       [4]float64{148.267e9, 392.331e9, 900.472e9, 1.867e12},
		Gems:         [4]float64{98209976, 190.664e6, 341.075e6, 572.880e6},
	},
	{
		ID:           7,
		Name:         "Warehouse",
		IconPath:     "egginc/ei_hab_icon_warehouse.png",
		BaseCapacity: 5e4,
		Prices:       [4]float64{17.795e12, 66.091e12, 194.147e12, 485.869e12},
		Gems:         [4]float64{2.968e9, 7.749e9, 17.259e9, 34.339e9},
	},
	{
		ID:           8,
		Name:         "Center",
		IconPath:     "egginc/ei_hab_icon_center.png",
		BaseCapacity: 1e5,
		Prices:       [4]float64{3.240e15, 8.752e15, 20.397e15, 42.893e15},
		Gems:         [4]float64{147.501e9, 308.995e9, 586.216e9, 1.032e12},
	},
	{
		ID:           9,
		Name:         "Bunker",
		IconPath:     "egginc/ei_hab_icon_bunker.png",
		BaseCapacity: 2e5,
		Prices:       [4]float64{250.213e15, 676.640e15, 1.578e18, 3.320e18},
		Gems:         [4]float64{4.088e12, 8.640e12, 16.499e12, 29.200e12},
	},
	{
		ID:           10,
		Name:         "Eggkea",
		IconPath:     "egginc/ei_hab_icon_eggkea.png",
		BaseCapacity: 5e5,
		Prices:       [4]float64{32.568e18, 122.013e18, 360.275e18, 904.176e18},
		Gems:         [4]float64{172.448e12, 473.363e12, 1.090e15, 2.220e15},
	},
	{
		ID:           11,
		Name:         "HAB 1000",
		IconPath:     "egginc/ei_hab_icon_hab1k.png",
		BaseCapacity: 1e6,
		Prices:       [4]float64{6.045e21, 16.323e21, 37.997e21, 79.803e21},
		Gems:         [4]float64{9.909e15, 21.011e15, 40.205e15, 71.211e15},
	},
	{
		ID:           12,
		Name:         "Hangar",
		IconPath:     "egginc/ei_hab_icon_hanger.png",
		BaseCapacity: 2e6,
		Prices:       [4]float64{463.301e21, 1.249e24, 2.904e24, 6.096e24},
		Gems:         [4]float64{284.720e15, 602.995e15, 1.153e18, 2.040e18},
	},
	{
		ID:           13,
		Name:         "Tower",
		IconPath:     "egginc/ei_hab_icon_tower.png",
		BaseCapacity: 5e6,
		Prices:       [4]float64{59.264e24, 220.221e24, 639.288e24, 1.577e27},
		Gems:         [4]float64{12.011e18, 32.853e18, 75.403e18, 153.035e18},
	},
	{
		ID:           14,
		Name:         "HAB 10,000",
		IconPath:     "egginc/ei_hab_icon_hab10k.png",
		BaseCapacity: 1e7,
		Prices:       [4]float64{12.467e27, 37.869e27, 95.965e27, 214.605e27},
		Gems:         [4]float64{794.835e18, 1.872e21, 3.867e21, 7.259e21},
	},
	{
		ID:           15,
		Name:         "Eggtopia",
		IconPath:     "egginc/ei_hab_icon_eggtopia.png",
		BaseCapacity: 2.5e7,
		Prices:       [4]float64{2.931e30, 12.853e30, 41.933e30, 112.893e30},
		Gems:         [4]float64{56.605e21, 175.232e21, 433.965e21, 926.747e21},
	},
	{
		ID:           16,
		Name:         "Monolith",
		IconPath:     "egginc/ei_hab_icon_monolith.png",
		BaseCapacity: 5e7,
		Prices:       [4]float64{2.800e33, 15.619e33, 59.243e33, 175.352e33},
		Gems:         [4]float64{10.928e24, 40.507e24, 112.595e24, 260.643e24},
	},
	{
		ID:           17,
		Name:         "Planet Portal",
		IconPath:     "egginc/ei_hab_icon_portal.png",
		BaseCapacity: 1e8,
		Prices:       [4]float64{8.573e36, 58.683e36, 238.757e36, 718.323e36},
		Gems:         [4]float64{5.694e27, 27.427e27, 88.837e27, 227.269e27},
	},
	{
		ID:           18,
		Name:         "Chicken Universe",
		IconPath:     "egginc/ei_hab_icon_chicken_universe.png",
		BaseCapacity: 6e8,
		Prices:       [4]float64{321.637e39, 3.589e42, 19.424e42, 71.237e42},
		Gems:         [4]float64{52.512e30, 389.347e30, 1.579e33, 4.640e33},
	},
}

// IsPortalHab returns true if the hab is a portal hab
func IsPortalHab(hab Hab) bool {
	return hab.ID >= 17
}
