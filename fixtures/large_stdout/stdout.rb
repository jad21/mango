#!/usr/bin/env ruby

$stdout.sync = true

10.times do |i|
  puts "sample log message... \n" * rand(i*1000) + "ok - #{i} \n"
  sleep 1
end

puts "finish!"
