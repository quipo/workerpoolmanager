#!/usr/bin/php
<?php
/**
 * Example of a task that fails the setup phase and terminates.
 * Since it doesn't send keep-alives to the manager, the manager
 * should detect it as stalled/dead, and start a new worker, trying to
 * restore the desired cardinality of workers.
 */

require __DIR__ .'/PoolWorker.php';

/**
 * Implement the setUpTask() and runTask() methods of the worker process
 *
 */
class ShortTask extends PoolWorker
{
	protected function setUpTask() {
		return false;
	}

	protected function runTask() {
		// $this->terminate is set when catching a SIGTERM
		while (!$this->terminate) {
			echo "Running!\n";
			sleep(1);
		}
	}
}

/** 
 * Run the task as cli script
 */
try {
	$config = array('keepalive' => array('address'=> 'tcp://localhost:5591'));
	$task = new ShortTask($config);
	$task->run();
} catch (Exception $e) {
	$msg = $e->getMessage().PHP_EOL.$e->getTraceAsString();
	echo $msg;
	error_log($msg);
	exit(1);
}

exit(0);