# Text comparison

The line engine uses Myers' shortest edit path (O((N+M)D) time), with common
prefix/suffix stripping and compact diagonal traces. This gives small edits
linear behavior without a third-party dependency. A trace limit of 8 million
integers bounds memory for unrelated inputs; those inputs produce one valid,
non-minimal replacement. Repeated-line tie breaking prefers deletion. It does
not reproduce IntelliJ's subsequent boundary-shifting/readability heuristics.

Lines preserve the final empty line, as IntelliJ's LineOffsets does. Ranges are
zero-based and half-open. Default compares exact text; ignore whitespace removes
Unicode whitespace within each line; ignore trailing whitespace removes only
line suffix whitespace. The latter intentionally differs from IntelliJ's
TRIM_WHITESPACES (which also removes leading whitespace).

Replacement lines are paired by ordinal position for inner highlighting. Words
are runs of Unicode letters/digits/underscore, whitespace runs, and individual
punctuation. The same edit engine compares tokens; spans refer to original UTF-8
bytes. This follows the comparable ByWord cases, without IntelliJ's language-
specific punctuation grouping. The tests identify the original Kotlin cases.

The merge model compares base-to-left and base-to-right, unions overlapping
base ranges, and merges isolated one-sided or identical changes automatically.
Adjacent changes on opposite sides are kept separate; an insertion on a range
boundary is grouped conservatively with that range. Unchosen conflicting chunks
keep the base text until the user accepts a side or explicitly edits the result.
