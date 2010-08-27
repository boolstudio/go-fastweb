{.section name}
Name: {name}<br/>
Manufacturer: {brand}<br/>
{.section image}
<img src="{image}"><br/>
{.end}
{.section features}
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
<form method="post" enctype="multipart/form-data">
<!--form action="ah64?def=xyz" method="post"-->
<input type="text" name="entry1" value="abc">
<input type="text" name="entry2" value="abc">
<input type="text" name="entry3" value="abc">
<input type="text" name="entry4" value="abc">
<input type="text" name="entry5" value="abc">
<input type="text" name="entry6" value="abc">
<input type="text" name="entry7" value="abc">
<input type="text" name="entry8" value="abc">
<input type="text" name="entry9" value="abc">
<input type="text" name="あsjkdfじゃklsdfjkl" value="abc">
<input type="file" name="hohoho">
<input type="submit" value="submit">
</form>
