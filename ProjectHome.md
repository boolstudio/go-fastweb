fastweb aims to be a simple, small and clean MVC framework for Google's Go programming language.

# Sample Usage #

You can find a very basic example in the "example" directory and the [demo page](http://dev.ivanwong.org/products/view/ah64). Basically the controller is implemented as follow:

```
package main

import (
        "fastweb"
        "os"
)

type Products struct {
        fastweb.Controller
        Name string
        Brand string
        Features []string
        Specifications []string
        Image string
}

func (p *Products) View(id string) os.Error {
        if id == "ah64" {
                p.Name = "RC Apache AH64 4-Channel Electric Helicoper"
                p.Brand = "Colco"
                p.Features = []string{
                        "4 channel radio control duel propeller system",
                        "Full movement controll: forward, backward, left, right, up and down",
                        "Replica design",
                        "Revolutionary co-axial rotor technology",
                }
                p.Specifications = []string{
                        "Dimensions: L 16 Inches X W 5.5 Inches x H 6.5 Inches",
                        "Battery Duration: 10 min",
                        "Range: 120 Feet",
                }
                p.Image = "/img/ah64.jpg"
        }
        return nil
}

func main() {
        a := fastweb.NewApplication()
        a.RegisterController(&Products{})
        a.Run(":12345")
}
```

and the template of the page body (example/views/products/view.tpl):

```
{.section Name}
Name: {Name}<br/>
Manufacturer: {Brand}<br/>
{.section Image}
<img src="{Image}"><br/>
{.end}
{.section Features}
Features:<br/>
<ul>
{.repeated section @}
<li>{@}</li>
{.end}
</ul>
{.end}
{.section specifications}
Specifications:<br/>
<ul>
{.repeated section @}
<li>{@}</li>
{.end}
</ul>
{.end}
{.or}
No product was found.
{.end}
```

## Sample Lighttpd Config ##

```
$HTTP["host"] =~ "" {
        server.document-root = "/home/ivan/go-fastweb/example/htdocs/"
        server.error-handler-404 = "/dispatch.fcgi"
        fastcgi.server = (
                ".fcgi" => ( "localhost" => (
                        "host" => "127.0.0.1",
                        "port" => 12345,
                        "check-local" => "disable",
                 )))
}
```

## Sample Apache Config ##

```
<VirtualHost *:80>
        ServerName      fastweb
        DocumentRoot    /home/ivan/go-fastweb/example/htdocs/

        ErrorLog /var/log/apache2/fastweb.error.log
        LogLevel warn
        CustomLog /var/log/apache2/fastweb.access.log combined
        ServerSignature On

        AddHandler fastcgi-script .fcgi
        FastCgiExternalServer /home/ivan/go-fastweb/example/htdocs/dispatch.fcgi -host 127.0.0.1:12345
        RewriteEngine On
        RewriteCond %{DOCUMENT_ROOT}%{REQUEST_FILENAME} !-f
        RewriteRule ^(.*)$ /dispatch.fcgi [QSA,L]
</VirtualHost>
```