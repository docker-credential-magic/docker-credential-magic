import subprocess


class Sh(object):
    def run_command(self, command, detach=False, hide_debug=False):
        process = subprocess.Popen(['/bin/bash', '-xc', command],
                                   stdout=subprocess.PIPE,
                                   stderr=subprocess.STDOUT)
        if not detach:
            stdout = process.communicate()[0].strip().decode()
            self.rc = process.returncode
            tmp = []
            for x in stdout.split('\n'):
                is_debug_line = x.startswith('+ ')
                if not is_debug_line or not hide_debug:
                    print(x)
                    # Remove debug lines that start with "+ "
                if not is_debug_line:
                    tmp.append(x)
            self.stdout = '\n'.join(tmp)

    def return_code_should_be(self, expected_rc):
        if int(expected_rc) != self.rc:
            raise AssertionError('Expected return code to be "%s" but was "%s".'
                                 % (expected_rc, self.rc))

    def should_pass(self, cmd):
        self.run_command(cmd)
        self.return_code_should_be(0)

    def should_pass_no_output(self, cmd):
        self.run_command(cmd, hide_debug=True)
        self.return_code_should_be(0)
