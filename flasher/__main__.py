import queue
import threading
import time
import tkinter as tk
from tkinter import ttk, messagebox, filedialog
from pathlib import Path
import webbrowser
from .device import Device
from . import __version__
from .messages import error_message

root = tk.Tk()
root.title(f'Paperclip Flasher {__version__} · Preview')
root.geometry('650x520')
frame = ttk.Frame(root, padding=24);frame.pack(fill='both', expand=True)
ttk.Label(frame, text='Paperclip Flasher', font=('', 22, 'bold')).pack(anchor='w')
ttk.Label(frame, text='Original KDBox · Windows and Linux · no firmware upgrade', wraplength=590).pack(anchor='w', pady=10)
values = {}
for key, label, secret in [('host','Miner IP address',False),('ssh','SSH root password',True),('web','Dashboard password',True)]:
    ttk.Label(frame,text=label).pack(anchor='w',pady=(10,2))
    values[key]=tk.StringVar()
    ttk.Entry(frame,textvariable=values[key],show='•' if secret else '').pack(fill='x')
status=tk.StringVar(value='Enter your miner details. Only this device will be changed.')
ttk.Label(frame,textvariable=status,wraplength=590).pack(anchor='w',pady=15)
events=queue.Queue()
busy=False

def progress(text):
    events.put(lambda text=text: status.set(text))

def finish():
    global busy
    busy=False
    button.configure(state='normal');restore_button.configure(state='normal')

def close_window():
    if busy:
        messagebox.showwarning('Operation in progress', 'Please leave this window open until the operation finishes. Interrupting installation or recovery can leave the miner needing repair.')
        return
    root.destroy()
root.protocol('WM_DELETE_WINDOW', close_window)

def confirm_host(host,fingerprint):
    done=threading.Event();result=[]
    def ask():
        result.append(messagebox.askyesno('Confirm miner identity',f'{host}\nSSH fingerprint:\n{fingerprint}\n\nIs this your miner?'))
        done.set()
    events.put(ask);done.wait();return result[0]

def run():
    global busy
    data={k:(v.get() if k in ['ssh','web'] else v.get().strip()) for k,v in values.items()}
    if not messagebox.askyesno('Device modification risk','Use at your own risk. Firmware-related changes can brick the miner or damage hardware. Rollback cannot guarantee recovery. Back up and install anyway?'):return
    busy=True
    button.configure(state='disabled');restore_button.configure(state='disabled')
    def work():
        device=None
        try:
            device=Device(data['host'],data['ssh'],confirm_host,web_password=data['web'],progress=progress)
            repo=Path(__file__).resolve().parents[1]
            backup=Path.home()/'PaperclipFlasher'/'backups'/time.strftime('%Y%m%d-%H%M%S')
            device.install(repo,data['web'],backup,progress)
            events.put(lambda:webbrowser.open('http://'+data['host']+'/'))
        except Exception as exc:
            progress('Stopped: '+error_message(exc, data['ssh'], data['web']))
        finally:
            if device:device.close()
            events.put(finish)
    threading.Thread(target=work,daemon=True).start()

button=ttk.Button(frame,text='Back up & install Paperclip',command=run);button.pack(anchor='w')
def restore():
    global busy
    folder=filedialog.askdirectory(title='Choose this miner’s recovery backup')
    if not folder or not messagebox.askyesno('Restore backup','Restore this miner’s saved configuration and restart it?'):return
    busy=True
    host=values['host'].get().strip();password=values['ssh'].get();web_password=values['web'].get();button.configure(state='disabled');restore_button.configure(state='disabled')
    def work():
        device=None
        try:
            device=Device(host,password,confirm_host,web_password=web_password,progress=progress);device.restore(folder);events.put(lambda:status.set('Backup restored. Miner is restarting.'))
        except Exception as exc:
            progress('Restore stopped: '+error_message(exc, password, web_password)+' Recovery may be incomplete; do not reboot or assume mining has resumed.')
        finally:
            if device:device.close()
            events.put(finish)
    threading.Thread(target=work,daemon=True).start()
restore_button=ttk.Button(frame,text='Restore a recovery backup',command=restore);restore_button.pack(anchor='w',pady=10)
def pump():
    while not events.empty():events.get()()
    root.after(100,pump)
pump();root.mainloop()
