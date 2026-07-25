using System;
using System.Threading.Tasks;
using DnsClient;

class Program
{
    static async Task Main()
    {
        var endpoint = new System.Net.IPEndPoint(System.Net.IPAddress.Parse("127.0.0.1"), 5300);
        var lookup = new LookupClient(endpoint);
        var result = await lookup.QueryAsync("8.8.8.8.geons", QueryType.TXT);

        foreach (var record in result.Answers.TxtRecords())
        {
            Console.WriteLine(string.Join(" ", record.Text));
        }
    }
}
