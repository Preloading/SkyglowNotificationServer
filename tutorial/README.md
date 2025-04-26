# Skyglowes Notifications Tutorial

# IMPORTANT NOTE: THIS TUTORIAL WAS WRITTEN FOR WHAT IS CURRENTLY A BETA!!!!

## You will need:
- A computer that can run the skyglow server
- Python3 installed on that computer
- A wifi network that your phone & your computer is both on

## Steps:

1. Create a folder somewhere. This is where the server will be hosted. Remember this location!!!

2. Download the server file (it should have a .py extention), and name it main.py. Your directory should now look like this:
![image](2.png)
3. Right click, Open in Terminal
![image](3.png)
(sorry for quality)
4. Type `python -m venv .venv` into the terminal window. It make take a bit to finish. Note: In terminal rightclick is copy if you have text highlighted, and paste if not
5. Activate the venv. This depends on your operating system, and shell.
<table class="docutils align-default">
<thead>
<tr class="row-odd"><th class="head"><p>Platform</p></th>
<th class="head"><p>Shell</p></th>
<th class="head"><p>Command to activate virtual environment</p></th>
</tr>
</thead>
<tbody>
<tr class="row-even"><td rowspan="4"><p>POSIX</p></td>
<td><p>bash/zsh</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">$</span> <span class="pre">source</span> <em><span class="pre">.venv</span></em><span class="pre">/bin/activate</span></code></p></td>
</tr>
<tr class="row-odd"><td><p>fish</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">$</span> <span class="pre">source</span> <em><span class="pre">.venv</span></em><span class="pre">/bin/activate.fish</span></code></p></td>
</tr>
<tr class="row-even"><td><p>csh/tcsh</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">$</span> <span class="pre">source</span> <em><span class="pre">.venv</span></em><span class="pre">/bin/activate.csh</span></code></p></td>
</tr>
<tr class="row-odd"><td><p>pwsh</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">$</span> <em><span class="pre">.venv</span></em><span class="pre">/bin/Activate.ps1</span></code></p></td>
</tr>
<tr class="row-even"><td rowspan="2"><p>Windows</p></td>
<td><p>cmd.exe</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">C:\&gt;</span> <em><span class="pre">.venv</span></em><span class="pre">\Scripts\activate.bat</span></code></p></td>
</tr>
<tr class="row-odd"><td><p>PowerShell</p></td>
<td><p><code class="samp docutils literal notranslate"><span class="pre">PS</span> <span class="pre">C:\&gt;</span> <em><span class="pre">.venv</span></em><span class="pre">\Scripts\Activate.ps1</span></code></p></td>
</tr>
</tbody>
</table>

(stolen directly from [https://docs.python.org/3/library/venv.html](https://docs.python.org/3/library/venv.html), thanks!)

6. Switching to your phone, install Skyglow Notification package. (atm the skyglow repo is outdated)

7. Go to Settings -> Skyglow Notifications -> Generate SSL Certificate

8. Once it finishes, open iFile, and click the wifi icon on the bottom

9. Go to the phone's iFile website (the website is listed under Accepting Connections at)

10. Navigate to /Library/PreferenceBundles/SkyglowNotificationsDaemonSettings.bundle/Keys, and download all the files into the directory you made above on your computer.
- Your directory should now look like this:
![image](10.png)

11. In the terminal window you opened previously, run `pip -r requirements.txt`. If that does not work, run `pip install flask pycryptodome`

12. Run `python main.py`

13. Go back to Settings -> Skyglow Notifications

14. Input the address of your computer & port 7373 into the Notification Server Address
- Your computer IP can be found in between http:// and :7878
![image](14.png)

15. Click Reload Daemon

16. Leave the Skyglow Notification settings menu, then reopen it.

17. In the terminal, it should be getting spammed. Press CTRL-C (no not to copy!) on the terminal

18. Copy the UUID from the logs. The UUID is what is after `Invalid UUID: `, and is also highlighted in the below photo
![image](18.png)

19. Rerun the `python main.py` (tip: you can press the up key to get the last thing you ran), but append ` --uuid=YOUR_UUID_FROM_ABOVE` (make sure there is a space between them)

20. Reload the daemon and exit the menu again.

21. Congrats! You've set up Skyglow Notifications. Now we will go into sending a notification.

22. Open a new terminal window, and enter the following

Windows
```bash
curl -X POST http://localhost:7878/send_data -H "Content-Type: application/json" -d "{\"sender\":\"SkyGlow\",\"message\":\"This is a notification\",\"topic\":\"com.atebits.Tweetie2\"}"
```
Anything else
```bash
curl -X POST http://localhost:5000/send_data \
    -H "Content-Type: application/json" \
    -d '{
        "sender": "SkyGlow",
        "message": "This is a notification",
        "topic": "com.atebits.Tweetie2"
    }'
```

23. Congrats! You just sent your first notification!

## FAQ:

### Q: Now how do I hook it up with X app?

A:  You spent a bunch of time developing something to send those notifications thru http://localhost:7878/send_data
