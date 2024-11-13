#!/bin/bash

#files=$(find . -type f -name "*deflector*.png" -or -name "*gusset*.png" -or -name "*compass*.png" -or -name "*metron*.png")
#files=$(find . -type f -name "afx_*feather*.png" )
#files=$(find . -type f -name "*_dust*.png" )
files=$(find . -type f -name "afx_*.png" )

for png_file in $files; do

	# If png_file doesn't exist skip
	if [ ! -f "$png_file" ]; then
		continue
	fi

	base_name=$(basename "$png_file")
	split_string=(${png_file//_/ })
	name=${split_string[2]}
	prelevel=${split_string[3]}

	# If the name is "of" (book_of_basan / beak_of_midas / light_of_eggendil) then...
	if [ "$name" == "of" ]; then
		name=${split_string[1]}
		prelevel=${split_string[4]}
	fi
	# ship_in_a_bottle
	if [ "$name" == "in" ]; then
		name=${split_string[1]}
		prelevel=${split_string[5]}
	fi

	# vial_martian_dust_2.png
	if [ "$name" == "martian" ]; then
		name=${split_string[3]}
		prelevel=${split_string[4]}
	fi

	# light_eggendil
	if [ "$name" == "eggendil" ]; then
		name=${split_string[1]}
		prelevel=${split_string[3]}
	fi


	split_lev=(${prelevel//./ })
	level=${split_lev[0]}

	letter="${name:0:1}"
	cap_name=$(tr '[:lower:]' '[:upper:]' <<< "$letter")

	echo "Processing $png_file to $name"

	type=("-" "L" "E" "R")
	for x in "${type[@]}"; do
		xx="${name}-T${level}${x}"

		# If type is "-" then we just copy this to a new name
		if [ "$x" == "-" ]; then
			xx="${name}-T${level}"
			cp ${png_file} ${xx}.png
			continue
		fi

		#output_gif=${base_name}-${type}.gif
		#output2_gif=${base_name}-${type}-a.gif
		#echo 		convert ./${x}.gif -coalesce null: -gravity center -geometry +0+0 -resize 256x256 ${png_file} \
		#	-layers composite -resize 62x62 -layers optimize -quality 25 -loop 0 ${xx}a.gif

		magick ./${x}.gif -coalesce null: -gravity center -geometry +0+0 -resize 256x256 ${png_file} \
			-layers composite -resize 62x62 -layers optimize -quality 25 -loop 0 ${xx}a.gif

		# Check the filesize of the new gif and if it's more than 256kb then we need to optimize it
		if [ $(stat -f%z "${xx}a.gif") -gt 262144 ]; then
			echo "  Optimizing 54x54 ${xx}a.gif"
			magick ./${x}.gif -coalesce null: -gravity center -geometry +0+0 -resize 256x256 ${png_file} \
				-layers composite -resize 54x54 -layers optimize -quality 25 -loop 0 ${xx}a.gif
		fi

		if [ $(stat -f%z "${xx}a.gif") -gt 262144 ]; then
			echo "    Optimizing 48x48 ${xx}a.gif"
			magick ./${x}.gif -coalesce null: -gravity center -geometry +0+0 -resize 256x256 ${png_file} \
				-layers composite -resize 48x48 -layers optimize -quality 25 -loop 0 ${xx}a.gif
		fi

	done
done
