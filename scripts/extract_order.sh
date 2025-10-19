#!/bin/bash


echo "{\"Order\":["

regex="\[.*\]"

# Collect all the directories in the ttbb-data directory and order them by date
directories=$(find ~/bots/ttbb-data -type d ! -name "images" -printf '%T@ %p\n' | sort -n | cut -d' ' -f2-)

# Iterate over the directories
for dir in $directories; do
    # Collect all the *.txt files in the current directory and order them by date
    files=$(find "$dir" -type f -name "*.txt" -printf '%T@ %p\n' | sort -n | cut -d' ' -f2-)

    # Iterate over the files
    for file in $files; do
        # Use grep with a regular expression to find lines containing the specified data
        #echo $file
        #data=$(grep -oh '"Order":\[.*\]' "$file" | grep -oh "\[.*\]")
        data=$(grep -oh '"Order":\[.*\]' "$file" | grep -oh "\[.*\]" )

        if [[ $data =~ $regex ]]
        then
            echo $data ","
        fi

    done
done

# Last empty array to prevent a JSON error
echo "[]]}"


