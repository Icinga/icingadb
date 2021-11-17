#!/usr/bin/perl -pi

use warnings;
use strict;
use autodie qw(:all);

if (/^ ?(?:\w+ )?Copyright / || /^All rights reserved\.$/ || /^(?:The )?\S+ License(?: \(.+?\))?$/ || /^$/) {
    $_ = ""
}

s/Google Inc\./the copyright holder/g
