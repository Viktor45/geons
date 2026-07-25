require 'resolv'

resolver = Resolv::DNS.new(nameserver: ['127.0.0.1:5300'])
answers = resolver.getresources('8.8.8.8.geons', Resolv::DNS::Resource::IN::TXT)
answers.each { |answer| puts answer.data.join(" ") }
