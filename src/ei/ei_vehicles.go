package ei

// Vehicles

type vehicleType struct {
	ID           uint32
	Name         string
	BaseCapacity float64 // Unupgraded shipping capacity per second.
}

var vehicleTypes = map[uint32]vehicleType{
	0: {
		ID:           0,
		Name:         "Trike",
		BaseCapacity: 5e3,
	},
	1: {
		ID:           1,
		Name:         "Transit Van",
		BaseCapacity: 15e3,
	},
	2: {
		ID:           2,
		Name:         "Pickup",
		BaseCapacity: 50e3,
	},
	3: {
		ID:           3,
		Name:         "10 Foot",
		BaseCapacity: 100e3,
	},
	4: {
		ID:           4,
		Name:         "24 Foot",
		BaseCapacity: 250e3,
	},
	5: {
		ID:           5,
		Name:         "Semi",
		BaseCapacity: 500e3,
	},
	6: {
		ID:           6,
		Name:         "Double Semi",
		BaseCapacity: 1e6,
	},
	7: {
		ID:           7,
		Name:         "Future Semi",
		BaseCapacity: 5e6,
	},
	8: {
		ID:           8,
		Name:         "Mega Semi",
		BaseCapacity: 15e6,
	},
	9: {
		ID:           9,
		Name:         "Hover Semi",
		BaseCapacity: 30e6,
	},
	10: {
		ID:           10,
		Name:         "Quantum Transporter",
		BaseCapacity: 50e6,
	},
	11: {
		ID:           11,
		Name:         "Hyperloop Train",
		BaseCapacity: 50e6,
	},
}

func isHoverVehicle(vehicle vehicleType) bool {
	return vehicle.ID >= 9
}

func isHyperloop(vehicle vehicleType) bool {
	return vehicle.ID == 11
}
