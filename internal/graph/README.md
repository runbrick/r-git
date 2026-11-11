# Commit graph layout

`NewLinearGraph` and `BuildLayout` port the permanent graph and lane assignment
algorithms from IntelliJ's VCS log graph. Inputs use dense row indexes as node
IDs; parent IDs refer to indexes in the same newest-first list. `BuildLayout`
orders heads with the supplied comparator and assigns each reachable line the
same lane and head as the Java implementation.

The printer package generates row nodes and edges, including visible portions
and arrows for collapsed long edges. Build a `PrintElementGenerator`, call
`PrepareRows` once, then reuse `PreparedRows.NewBuffer()` and `PreparedRows.GetRow`
for scrolling. The query copies into caller-owned slices and performs no heap
allocation when using the provided buffer.

On a 64-bit Go build, the permanent graph and lane layout use about 12.125 bytes
per commit for a linear history: 4 bytes for the adjacency offset, 8 bytes for
the lane index, and one bit for the simple-edge flag. Non-linear parent edges
add 8 bytes each to the packed edge indexes, and head metadata adds 8 bytes per
head. `PreparedRows` adds 32 bytes per commit for row ranges, 48 bytes per
stored node element, and 104 bytes per stored edge element, plus slice capacity
slack and allocator overhead. Stored edge count depends on graph topology and
long-edge visibility; this representation deliberately trades memory for
allocation-free scrolling queries.

Branch collapse/expand and graph filtering remain follow-up work. The port only
handles the permanent graph and layout plus row-element generation.
