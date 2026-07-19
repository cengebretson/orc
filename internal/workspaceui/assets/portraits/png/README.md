# Portrait PNGs

Drop portrait images here named after their class (lowercase):

- `warrior.png`
- `monk.png`
- `wizard.png`
- `bard.png`
- `ranger.png`
- `priest.png`
- `rogue.png`
- `barbarian.png`
- `adventurer.png`

The Workspace dashboard renders them as truecolor half-block art (▀) when COLORTERM=truecolor.
Falls back to ASCII art from `../` if a PNG is missing or truecolor is unavailable.

Source sheets live in `../sheets/` — `characters1.png` (hero classes, cropped
into the PNGs here) and `characters2.png` (orc classes, not yet cropped). The
`sheets/` folder is outside the go:embed pattern, so it does not grow the binary.
