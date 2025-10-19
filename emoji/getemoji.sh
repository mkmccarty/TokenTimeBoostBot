#!/bin/bash

# Name of the input file
input_file="botemoji.txt"

# Read the file line by line
while IFS=, read -r name url; do
    # Ignore comment lines starting with # 
    if [[ $name == \#* ]]; then 
        continue 
    fi

    # Extract the file name and extension, ignoring the parameters
    file_ext=$(echo "$url" | grep -oE '\.[a-z]+(\?|$)' | grep -oE '[a-z]+')
    
    # Handle both png and gif files and their parameters
    if [ ! -f "${name}.${file_ext}" ]; then
        # Download the image and rename it
        wget --quiet -O "${name}.${file_ext}" "${url}"
    else
        echo "File ${name}.${file_ext} already exists, skipping download."
    fi
done < "$input_file"
