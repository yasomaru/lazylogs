# lazylogs demo - https://github.com/charmbracelet/vhs
# Run: vhs demo.tape

Output assets/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Build first
Hide
Type "go build -o lazylogs ./cmd/lazylogs"
Enter
Sleep 2s
Show

# Start lazylogs with sample data
Type "cat testdata/demo.jsonl | ./lazylogs"
Enter
Sleep 2s

# Scroll through logs
Sleep 1s
Type "k"
Sleep 300ms
Type "k"
Sleep 300ms
Type "k"
Sleep 300ms
Type "k"
Sleep 300ms
Type "k"
Sleep 1s

# Show detail view
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Enter
Sleep 2.5s
Escape
Sleep 1s

# Filter errors only
Type "1"
Sleep 2s

# Show all again
Type "0"
Sleep 1s

# Search
Type "/"
Sleep 500ms
Type "payment"
Enter
Sleep 2s

# Clear search
Escape
Sleep 1s

# Time filter
Type "t"
Sleep 1s
Type "1"
Sleep 2s

# Reset time filter
Type "t"
Sleep 500ms
Type "0"
Sleep 1s

# Column mode demo
Type "q"
Sleep 500ms
Type "cat testdata/demo.jsonl | ./lazylogs --columns time,level,msg,status,latency"
Enter
Sleep 3s

# Scroll in column mode
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Sleep 2s

Type "q"
Sleep 1s
