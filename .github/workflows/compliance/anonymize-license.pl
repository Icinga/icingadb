#!/usr/bin/perl -pi

use warnings;
use strict;
use autodie qw(:all);

if (/^ ?Copyright / || /^All rights reserved\.$/ || /^(?:The )?\S+ License(?: \(.+?\))?$/ || /^$/) {
    $_ = ""
}

s/Google Inc\./the copyright holder/g
