const { Resolver } = require('dns');

const resolver = new Resolver();
resolver.setServers(['127.0.0.1:5300']);

resolver.resolveTxt('8.8.8.8.geons', (err, records) => {
  if (err) {
    console.error(err);
    process.exit(1);
  }

  console.log(records);
});
