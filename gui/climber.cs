using Gtk;
using System;
using System.Diagnostics;

class ClimberApp : Window
{

    private Label _lbl;
    public static void Main() 
    {
        Application.Init();
        new ClimberApp();
        Application.Run();
    }

    public ClimberApp() : base("Fuck Censorship!")
    {
        SetDefaultSize(400, 200);
        SetPosition(WindowPosition.Center);
        DeleteEvent += delegate { Environment.Exit(0); };

        Fixed fix = new Fixed();

        _lbl = new Label("");

        Button startBtn = new Button("Start");
        startBtn.Sensitive = false;
        startBtn.Clicked += startBtn_Click;

        Button stopBtn = new Button("Stop");
        stopBtn.Sensitive = false;

        Button closeBtn = new Button(Stock.Close);

        fix.Put(_lbl, 0, 200);
        fix.Put(startBtn, 50, 50);
        fix.Put(stopBtn, 300, 50);
        fix.Put(closeBtn, 175, 125);

        Add(fix);

        ShowAll();
    }

    private void startBtn_Click(object sender, EventArgs e)
    {
        Process process = new Process();
        process.StartInfo.FileName = "../main";
        process.StartInfo.UseShellExecute = false;
        process.StartInfo.RedirectStandardOutput = true;
        process.StartInfo.RedirectStandardError = true;

        _lbl.Text = "Climb the Wall!";
        
        process.Start();
        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
        process.WaitForExit();
    }
}
