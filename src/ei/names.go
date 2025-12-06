package ei

// DurationTypeName maps duration type IDs to their names
var DurationTypeName = map[int32]string{
	0: "Short",
	1: "Standard",
	2: "Extended",
}

// DurationTypeNameAbbr maps duration type IDs to their abbreviated names
var DurationTypeNameAbbr = map[int32]string{
	0: "SH",
	1: "ST",
	2: "EX",
}

// ShipMaxStars maps ship type IDs to their maximum star levels
var ShipMaxStars = map[int32]int32{
	0:  0,
	1:  2,
	2:  3,
	3:  4,
	4:  4,
	5:  4,
	6:  5,
	7:  5,
	8:  6,
	9:  8,
	10: 8,
}

// ShipTypeName maps ship type IDs to their names
var ShipTypeName = map[int32]string{
	0:  "Chicken One",
	1:  "Chicken Nine",
	2:  "Chicken Heavy",
	3:  "BCR",
	4:  "Quintillion Chicken",
	5:  "Cornish-Hen Corvette",
	6:  "Galeggtica",
	7:  "Defihent",
	8:  "Voyegger",
	9:  "Henerprise",
	10: "Atreggies Henliner",
}

// ArtifactTypeName maps artifact type IDs to their names
var ArtifactTypeName = map[int32]string{
	0:     "Lunar Totem",
	1:     "Tachyon Stone",
	2:     "Tachyon Stone Fragment",
	3:     "Neodymium Medallion",
	4:     "Beak of Midas",
	5:     "Light of Eggendil",
	6:     "Demeters Necklace",
	7:     "Vial of Martian Dust",
	8:     "Gusset",
	9:     "The Chalice",
	10:    "Book of Basan",
	11:    "Phoenix Feather",
	12:    "Tungsten Ankh",
	17:    "Gold Meteorite",
	18:    "Tau Ceti Geode",
	21:    "Aurelian Brooch",
	22:    "Carved Rainstick",
	23:    "Puzzle Cube",
	24:    "Quantum Metronome",
	25:    "Ship in a Bottle",
	26:    "Tachyon Deflector",
	27:    "Interstellar Compass",
	28:    "Dilithium Monocle",
	29:    "Titanium Actuator",
	30:    "Mercurys Lens",
	31:    "Dilithium Stone",
	32:    "Shell Stone",
	33:    "Lunar Stone",
	34:    "Soul Stone",
	36:    "Quantum Stone",
	37:    "Terra Stone",
	38:    "Life Stone",
	39:    "Prophecy Stone",
	40:    "Clarity Stone",
	43:    "Solar Titanium",
	44:    "Dilithium Stone Fragment",
	45:    "Shell Stone Fragment",
	46:    "Lunar Stone Fragment",
	47:    "Soul Stone Fragment",
	48:    "Prophecy Stone Fragment",
	49:    "Quantum Stone Fragment",
	50:    "Terra Stone Fragment",
	51:    "Life Stone Fragment",
	52:    "Clarity Stone Fragment",
	10000: "No Target",
}

// ArtifactTypeNameVirtue maps artifact type IDs to their names for virtue artifacts
var ArtifactTypeNameVirtue = map[int32]string{
	0:  "Lunar Totem",
	1:  "Tachyon Stone",
	3:  "Neodymium Medallion",
	6:  "Demeters Necklace",
	8:  "Gusset",
	12: "Tungsten Ankh",
	17: "Gold Meteorite",
	18: "Tau Ceti Geode",
	23: "Puzzle Cube",
	27: "Interstellar Compass",
	33: "Lunar Stone",
	36: "Quantum Stone",
	43: "Solar Titanium",
}
