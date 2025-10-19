#!/bin/bash

# Define the strings for replacement
OLD_STRING="CxpVersion"
NEW_STRING="SeasonalScoring"

# Find all .go files in the current directory and its subdirectories,
# and use sed to perform the in-place replacement.
# The 'g' flag ensures all occurrences on a line are replaced.
# The 'i' flag with a suffix (e.g., '.bak') creates a backup;
# using 'i ""' or 'i' (depending on the system/sed version) modifies in place without a backup.
# For maximum compatibility, we'll use a common, portable approach with a temporary file.

find . -type f -name "*.go" -print0 | while IFS= read -r -d $'\0' file; do
    echo "Processing file: $file"
    # Use sed to replace the string and save the change in-place
    # The 'i' option in sed is system-dependent.
    # For GNU sed (common on Linux):
    # sed -i "s/${OLD_STRING}/${NEW_STRING}/g" "$file"

    # For macOS/BSD sed (which requires an argument to -i):
    sed -i '' "s/${OLD_STRING}/${NEW_STRING}/g" "$file"

    # If the above fails, you can use a portable method:
    # sed "s/${OLD_STRING}/${NEW_STRING}/g" "$file" > "$file.tmp" && mv "$file.tmp" "$file"
done

echo "Replacement complete in all .go files."
