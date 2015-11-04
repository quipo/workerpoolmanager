Name:      %{_package}
Version:   %{_version}
Release:   %{_release}%{?dist}
Summary:   Task / worker pool manager in Go

Group:     Applications/Services
License:   MIT
URL:       https://github.com/quipo/workerpoolmanager

BuildRoot: %{_tmppath}/%{name}-%{version}-%{release}-%(%{__id_u} -n)

Provides:  workerpoolmanager
Provides:  wpconsole
Provides:  wpmanager

%description
Task / worker pool manager in Go:
Start cli tasks automatically;
Maintain the desidered number of worker processes for each task;
Handle automatic restarts when a worker dies or stalls.

%build
(cd %{_current_directory} && make build)

%install
rm -rf $RPM_BUILD_ROOT
(cd %{_current_directory} && make install DESTDIR=$RPM_BUILD_ROOT)

%clean
rm -rf $RPM_BUILD_ROOT
(cd %{_current_directory} && make clean)

%files
%attr(-,root,root) %{_binpath}
%attr(-,root,root) %{_docpath}
%docdir %{_docpath}
%config(noreplace) %{_configpath}*

%changelog
* Tue Nov 03 2015 Nicola Asuni <nicola.asuni@datasift.com> 2.1.0-1
- First version
