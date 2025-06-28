#!/bin/bash

echo "=== DPoS Codebase Analysis ==="
echo "Analyzing for duplicated and unused code..."
echo

# Find all Go files
echo "1. Total Go files:"
find . -name "*.go" | wc -l
echo

# Find function definitions
echo "2. Function definitions by file:"
find . -name "*.go" -exec grep -l "func.*(" {} \; | while read file; do
    echo "$file: $(grep -c 'func.*(' "$file") functions"
done
echo

# Find potential duplicates
echo "3. Potential duplicate function names:"
grep -h "func.*(" *.go | grep -v "Test" | sed 's/func \([^(]*\)(.*/\1/' | sort | uniq -d
echo

# Find unused imports
echo "4. Checking for unused imports..."
go vet ./... 2>&1 | grep -E "(unused|unreachable)" || echo "No obvious unused code found by go vet"
echo

# Find TODO/FIXME comments
echo "5. TODO/FIXME comments:"
grep -r "TODO\|FIXME\|XXX\|HACK" . --include="*.go" || echo "No TODO/FIXME comments found"
echo

# Find commented out code
echo "6. Commented out code blocks:"
grep -r "// func\|// type\|// var" . --include="*.go" || echo "No commented out code found"
echo

# Find backup files
echo "7. Backup files:"
find . -name "*.backup" -o -name "*.bak" -o -name "*~" || echo "No backup files found"
echo

# Find duplicate binary files
echo "8. Binary files:"
ls -la *.db* 2>/dev/null | wc -l
echo

echo "=== Analysis Complete ===" 