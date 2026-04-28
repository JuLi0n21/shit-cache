# shit-cache

mirco programm to proxy files from an origin to ur hearts content, to either save on bandwidth or have them incase of origin downtime

## Usage:

pass the base64 encoded url u want to proxy to the webserver and ur done 

`cache.example.com/{base64encodedUrl}`

## Security:

if u plan to use this on a public domain u can limit the domains its allowed to fetch from with the  `ALLOWED_DOMAINS` enviorment variable, subdomains are INCLUDED by default, if none are provided every domain is fair game

`ALLOWED_DOMAINS=mycoolsite.com,example.com`

## Additional stuff

- this is abadonware and will not be maintained
- ai was used in the creation process
- docker images will not be provided, u can build them urself