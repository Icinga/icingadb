#!/usr/bin/perl

use warnings;
use strict;
use autodie qw(:all);

my @mods = <>;
chomp @mods;
s~^vendor/~~ for @mods;

@mods = grep m~^[^./]+\.~, @mods;
@mods = grep !m~^golang\.org/x(?:/|$)~, @mods;
@mods = grep !m~^github\.com/icinga/icingadb(?:/|$)~, @mods;
@mods = sort @mods;

my $lastMod = undef;

for (@mods) {
    # prefixed with last mod (e.g. "go.uber.org/zap/buffer" after "go.uber.org/zap"), so redundant
    next if defined($lastMod) && /$lastMod/;

    $lastMod = '^' . quotemeta("$_/");
    print "$_\n"
}
