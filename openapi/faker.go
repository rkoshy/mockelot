package openapi

// FakerJS contains the embedded Faker.js-like utilities for JavaScript mock generation
const FakerJS = `// Faker.js-like utilities for generating realistic mock data
const faker = {
    random: {
        uuid: () => crypto.randomUUID(),
        number: (min = 0, max = 100) => Math.floor(Math.random() * (max - min + 1)) + min,
        boolean: () => Math.random() > 0.5,
        arrayElement: (arr) => arr[Math.floor(Math.random() * arr.length)],
        float: (min = 0, max = 100, precision = 2) => {
            const val = Math.random() * (max - min) + min;
            return parseFloat(val.toFixed(precision));
        }
    },
    internet: {
        email: () => {
            const names = ['john', 'jane', 'alice', 'bob', 'charlie', 'diana'];
            const domains = ['example.com', 'test.com', 'demo.com', 'sample.org'];
            return faker.random.arrayElement(names) +
                   Math.floor(Math.random() * 1000) + '@' +
                   faker.random.arrayElement(domains);
        },
        url: () => {
            const protocols = ['https'];
            const domains = ['example.com', 'test.com', 'api.demo.com'];
            const paths = ['users', 'api', 'data', 'resources'];
            return faker.random.arrayElement(protocols) + '://' +
                   faker.random.arrayElement(domains) + '/' +
                   faker.random.arrayElement(paths) + '/' +
                   Math.random().toString(36).substring(7);
        },
        ipv4: () => Array(4).fill(0).map(() => Math.floor(Math.random() * 256)).join('.'),
        ipv6: () => Array(8).fill(0).map(() => Math.floor(Math.random() * 65536).toString(16).padStart(4, '0')).join(':'),
        domainName: () => {
            const prefixes = ['api', 'www', 'mail', 'dev', 'staging', 'prod'];
            const names = ['example', 'test', 'demo', 'sample'];
            const tlds = ['com', 'org', 'net', 'io'];
            return faker.random.arrayElement(prefixes) + '.' +
                   faker.random.arrayElement(names) + '.' +
                   faker.random.arrayElement(tlds);
        },
        username: () => {
            const adjectives = ['cool', 'super', 'mega', 'ultra'];
            const nouns = ['user', 'coder', 'dev', 'ninja'];
            return faker.random.arrayElement(adjectives) +
                   faker.random.arrayElement(nouns) +
                   Math.floor(Math.random() * 1000);
        }
    },
    date: {
        now: () => new Date().toISOString(),
        today: () => new Date().toISOString().split('T')[0],
        recent: (days = 7) => {
            const d = new Date();
            d.setDate(d.getDate() - Math.floor(Math.random() * days));
            return d.toISOString();
        },
        future: (days = 365) => {
            const d = new Date();
            d.setDate(d.getDate() + Math.floor(Math.random() * days));
            return d.toISOString();
        },
        past: (days = 365) => {
            const d = new Date();
            d.setDate(d.getDate() - Math.floor(Math.random() * days));
            return d.toISOString();
        },
        timestamp: () => Date.now(),
        timestampMs: () => Date.now()
    },
    lorem: {
        word: () => {
            const words = ['lorem', 'ipsum', 'dolor', 'sit', 'amet', 'consectetur',
                          'adipiscing', 'elit', 'sed', 'do', 'eiusmod', 'tempor'];
            return faker.random.arrayElement(words);
        },
        words: (count = 3) => {
            const result = [];
            for (let i = 0; i < count; i++) {
                result.push(faker.lorem.word());
            }
            return result.join(' ');
        },
        sentence: () => {
            const sentences = [
                'Lorem ipsum dolor sit amet, consectetur adipiscing elit.',
                'Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                'Ut enim ad minim veniam, quis nostrud exercitation ullamco.',
                'Duis aute irure dolor in reprehenderit in voluptate velit.'
            ];
            return faker.random.arrayElement(sentences);
        },
        paragraph: () => {
            return 'Lorem ipsum dolor sit amet, consectetur adipiscing elit. ' +
                   'Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. ' +
                   'Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.';
        }
    },
    name: {
        firstName: () => {
            const names = ['John', 'Jane', 'Alice', 'Bob', 'Charlie', 'Diana',
                          'Edward', 'Fiona', 'George', 'Helen'];
            return faker.random.arrayElement(names);
        },
        lastName: () => {
            const names = ['Smith', 'Johnson', 'Williams', 'Brown', 'Jones',
                          'Garcia', 'Miller', 'Davis', 'Rodriguez', 'Martinez'];
            return faker.random.arrayElement(names);
        },
        fullName: () => faker.name.firstName() + ' ' + faker.name.lastName()
    },
    address: {
        city: () => {
            const cities = ['New York', 'Los Angeles', 'Chicago', 'Houston', 'Phoenix',
                           'Philadelphia', 'San Antonio', 'San Diego', 'Dallas', 'San Jose'];
            return faker.random.arrayElement(cities);
        },
        country: () => {
            const countries = ['USA', 'Canada', 'Mexico', 'UK', 'France', 'Germany',
                             'Italy', 'Spain', 'Japan', 'Australia'];
            return faker.random.arrayElement(countries);
        },
        zipCode: () => {
            return String(faker.random.number(10000, 99999));
        },
        street: () => {
            const streets = ['Main St', 'Oak Ave', 'Maple Dr', 'Cedar Ln', 'Elm Rd'];
            return faker.random.number(1, 9999) + ' ' + faker.random.arrayElement(streets);
        }
    },
    company: {
        name: () => {
            const prefixes = ['Tech', 'Data', 'Cloud', 'Smart', 'Digital'];
            const suffixes = ['Corp', 'Inc', 'LLC', 'Solutions', 'Systems'];
            return faker.random.arrayElement(prefixes) +
                   faker.random.arrayElement(suffixes);
        }
    }
};
`
