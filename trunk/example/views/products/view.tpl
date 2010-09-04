<%.section name%>
Name: <%name%><br/>
Manufacturer: <%brand%><br/>
<%.section image%>
<img src="<%image%>"><br/>
<%.end%>
<%.section features%>
Features:<br/>
<ul>
<%.repeated section @%>
<li><%@%></li>
<%.end%>
</ul>
<%.end%>
<%.section specifications%>
Specifications:<br/>
<ul>
<%.repeated section @%>
<li><%@%></li>
<%.end%>
</ul>
<%.end%>
<%.or%>
No product was found.
<%.end%>
